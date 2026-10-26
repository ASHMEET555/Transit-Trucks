package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	PingInterval      = 20 * time.Second
	PongWait          = 60 * time.Second
	WriteWait         = 10 * time.Second
	SendChannelCap    = 64
	MaxConnsPerUser   = 5
	CloseSlowConsumer = 1013
	CloseTooManyConns = 4429
	CloseShutdown     = 1001
)

type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	userID        string
	role          string
	driverLicense string
	closed        bool
	mu            sync.Mutex
}

type Hub struct {
	dbURL          string
	pool           *pgxpool.Pool
	allowedOrigins map[string]bool
	logger         *slog.Logger

	mu          sync.RWMutex
	clients     map[*Client]bool
	userConnCnt map[string]int

	broadcastCh chan Event
	quitCh      chan struct{}
	readyCh      chan struct{}
	readyOnce    sync.Once
	cancelListen context.CancelFunc
	wg           sync.WaitGroup

	// Test hooks to customize timers if needed
	pingInterval time.Duration
	pongWait     time.Duration
}

func NewHub(dbURL string, pool *pgxpool.Pool, allowedOrigins []string, logger *slog.Logger) *Hub {
	originsMap := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originsMap[o] = true
	}

	h := &Hub{
		dbURL:          dbURL,
		pool:           pool,
		allowedOrigins: originsMap,
		logger:         logger,
		clients:        make(map[*Client]bool),
		userConnCnt:    make(map[string]int),
		broadcastCh:    make(chan Event, 256),
		quitCh:         make(chan struct{}),
		readyCh:        make(chan struct{}),
		pingInterval:   PingInterval,
		pongWait:       PongWait,
	}

	return h
}

// Start begins the dedicated PostgreSQL LISTEN loop and broadcast fan-out.
func (h *Hub) Start(ctx context.Context) {
	listenCtx, cancel := context.WithCancel(ctx)
	h.cancelListen = cancel

	h.wg.Add(1)
	go h.listenLoop(listenCtx)

	h.wg.Add(1)
	go h.dispatchLoop()
}

// Close stops the listener and gracefully closes all active client sockets.
func (h *Hub) Close() {
	select {
	case <-h.quitCh:
		return // already closed
	default:
		close(h.quitCh)
	}

	if h.cancelListen != nil {
		h.cancelListen()
	}

	h.mu.Lock()
	for client := range h.clients {
		_ = client.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(CloseShutdown, "Server shutting down"),
			time.Now().Add(WriteWait),
		)
		client.conn.Close()
		client.mu.Lock()
		if !client.closed {
			client.closed = true
			close(client.send)
		}
		client.mu.Unlock()
		delete(h.clients, client)
	}
	h.userConnCnt = make(map[string]int)
	h.mu.Unlock()

	h.wg.Wait()
}

// CheckOrigin verifies that the HTTP Origin matches ALLOWED_ORIGINS.
func (h *Hub) CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return h.allowedOrigins[origin]
}

// Upgrade upgrades an incoming HTTP request to a WebSocket connection after
// validating ticket, checking origin, and enforcing the per-user connection limit.
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request, driverLicenseLookup func(ctx context.Context, userID string) string) error {
	// 1. Ticket check: ticket is mandatory
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Missing ticket", nil)
		return fmt.Errorf("missing ticket")
	}

	// 2. Origin check: if Origin header is present, must be allowed
	origin := r.Header.Get("Origin")
	if origin != "" && !h.allowedOrigins[origin] {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Origin not allowed", nil)
		return fmt.Errorf("forbidden origin")
	}

	// 3. Consume ticket: atomically claims the single-use ticket
	userID, role, err := ConsumeTicket(r.Context(), h.pool, ticket)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired ticket", nil)
		return fmt.Errorf("invalid ticket: %w", err)
	}

	// 3. Check connection count before upgrade
	h.mu.Lock()
	currCount := h.userConnCnt[userID]
	if currCount >= MaxConnsPerUser {
		h.mu.Unlock()
		// Reject 6th connection: per spec, sixth receives close code 4429
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			msg := websocket.FormatCloseMessage(CloseTooManyConns, "Max connections reached for user")
			_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(WriteWait))
			_ = conn.Close()
		}
		return fmt.Errorf("too many connections")
	}
	h.userConnCnt[userID]++
	h.mu.Unlock()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.mu.Lock()
		h.userConnCnt[userID]--
		h.mu.Unlock()
		return fmt.Errorf("upgrade failed: %w", err)
	}

	var dLic string
	if driverLicenseLookup != nil {
		dLic = driverLicenseLookup(r.Context(), userID)
	}

	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan []byte, SendChannelCap),
		userID:        userID,
		role:          role,
		driverLicense: dLic,
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	go client.writePump()
	go client.readPump()

	return nil
}

func (c *Client) readPump() {
	defer func() {
		c.hub.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.hub.pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.hub.quitCh:
			return
		}
	}
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		c.mu.Lock()
		if !c.closed {
			c.closed = true
			close(c.send)
		}
		c.mu.Unlock()

		h.userConnCnt[c.userID]--
		if h.userConnCnt[c.userID] <= 0 {
			delete(h.userConnCnt, c.userID)
		}
	}
}

func (h *Hub) dispatchLoop() {
	defer h.wg.Done()

	for {
		select {
		case ev := <-h.broadcastCh:
			h.fanOutEvent(ev)
		case <-h.quitCh:
			return
		}
	}
}

func (h *Hub) fanOutEvent(ev Event) {
	h.mu.RLock()
	clientsList := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clientsList = append(clientsList, c)
	}
	h.mu.RUnlock()

	// Extract target driver license from event IDs if present for ownership check
	var targetDriverLicense string
	if licVal, ok := ev.IDs["driver_license"]; ok && licVal != nil {
		targetDriverLicense = fmt.Sprint(licVal)
	}

	sanitizedIDs := SanitizedClientPayload(ev.IDs)
	clientEvent := Event{
		Type:       ev.Type,
		IDs:        sanitizedIDs,
		OccurredAt: ev.OccurredAt,
	}
	msgBytes, err := json.Marshal(clientEvent)
	if err != nil {
		return
	}

	for _, c := range clientsList {
		ownsTrip := (c.driverLicense != "" && targetDriverLicense != "" && c.driverLicense == targetDriverLicense)
		if !CanReceive(ev.Type, c.role, ownsTrip) {
			continue
		}

		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			continue
		}

		select {
		case c.send <- msgBytes:
			c.mu.Unlock()
		default:
			c.mu.Unlock()
			// Bounded send channel is full (slow consumer) -> drop socket with close code 1013
			go func(slowClient *Client) {
				msg := websocket.FormatCloseMessage(CloseSlowConsumer, "Slow consumer dropped")
				_ = slowClient.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(WriteWait))
				slowClient.conn.Close()
				h.removeClient(slowClient)
			}(c)
		}
	}
}

// listenLoop maintains a dedicated session-held connection to Postgres running LISTEN fleet_events.
func (h *Hub) listenLoop(ctx context.Context) {
	defer h.wg.Done()

	for {
		select {
		case <-h.quitCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		conn, err := pgx.Connect(ctx, h.dbURL)
		if err != nil {
			select {
			case <-time.After(1 * time.Second):
				continue
			case <-h.quitCh:
				return
			case <-ctx.Done():
				return
			}
		}

		_, err = conn.Exec(ctx, "LISTEN "+ChannelFleetEvents)
		if err != nil {
			_ = conn.Close(ctx)
			time.Sleep(1 * time.Second)
			continue
		}
		h.readyOnce.Do(func() {
			close(h.readyCh)
		})

		// Read notifications until disconnect
		for {
			notification, err := conn.WaitForNotification(ctx)
			if err != nil {
				_ = conn.Close(ctx)
				break
			}

			var ev Event
			if err := json.Unmarshal([]byte(notification.Payload), &ev); err == nil {
				select {
				case h.broadcastCh <- ev:
				default:
					// Broadcast channel congested
				}
			}
		}
	}
}

// Ready returns a channel that is closed once the dedicated LISTEN connection is established.
func (h *Hub) Ready() <-chan struct{} {
	return h.readyCh
}

// ActiveClientCount returns the number of currently connected WebSocket clients.
func (h *Hub) ActiveClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ActiveClientCountByRole returns the number of connected WebSocket clients per role.
func (h *Hub) ActiveClientCountByRole() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	counts := make(map[string]int)
	for c := range h.clients {
		counts[c.role]++
	}
	return counts
}
