package middleware

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userContextKey string

const UserKey userContextKey = "authenticated_user"

type routeContextKey string

const RoutePatternKey routeContextKey = "route_pattern"

func SetRoutePattern(r *http.Request, pattern string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), RoutePatternKey, pattern))
}

func GetRoutePattern(r *http.Request) string {
	if p, ok := r.Context().Value(RoutePatternKey).(string); ok && p != "" {
		return p
	}
	if r.Pattern != "" {
		parts := strings.SplitN(r.Pattern, " ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return r.Pattern
	}
	return "unmatched"
}

func GetUser(r *http.Request) *auth.SessionUser {
	if u, ok := r.Context().Value(UserKey).(*auth.SessionUser); ok {
		return u
	}
	return nil
}

func WithUser(ctx context.Context, u *auth.SessionUser) context.Context {
	return context.WithValue(ctx, UserKey, u)
}

// RequestTimeout applies a context deadline to standard HTTP requests.
// WebSocket endpoints are excluded. If the deadline expires before completion,
// it ensures a 503 REQUEST_TIMEOUT response is written.
func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/ws" {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			rWithTimeout := r.WithContext(ctx)
			done := make(chan struct{})
			panicVal := make(chan any, 1)

			tw := &timeoutWriter{w: w}

			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicVal <- rec
					}
					close(done)
				}()
				next.ServeHTTP(tw, rWithTimeout)
			}()

			select {
			case p := <-panicVal:
				panic(p)
			case <-done:
				tw.mu.Lock()
				defer tw.mu.Unlock()
				tw.copyToRealLocked()
			case <-ctx.Done():
				tw.mu.Lock()
				defer tw.mu.Unlock()
				tw.timedOut = true
				httpx.WriteError(w, rWithTimeout, http.StatusServiceUnavailable, "REQUEST_TIMEOUT", "Request execution timed out.", nil)
			}
		})
	}
}

type timeoutWriter struct {
	w           http.ResponseWriter
	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
	code        int
	buf         bytes.Buffer
	header      http.Header
}

func (tw *timeoutWriter) Header() http.Header {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.header == nil {
		tw.header = make(http.Header)
		for k, v := range tw.w.Header() {
			tw.header[k] = slices.Clone(v)
		}
	}
	return tw.header
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.code = code
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return len(b), nil
	}
	if !tw.wroteHeader {
		tw.wroteHeader = true
		tw.code = http.StatusOK
	}
	return tw.buf.Write(b)
}

func (tw *timeoutWriter) Flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return
	}
	tw.copyToRealLocked()
	if f, ok := tw.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (tw *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := tw.w.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (tw *timeoutWriter) copyToRealLocked() {
	if tw.timedOut {
		return
	}
	for k, v := range tw.header {
		tw.w.Header()[k] = v
	}
	if tw.wroteHeader && tw.code != 0 {
		tw.w.WriteHeader(tw.code)
	}
	if tw.buf.Len() > 0 {
		_, _ = tw.w.Write(tw.buf.Bytes())
		tw.buf.Reset()
	}
}

// 1. Recover middleware
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := httpx.RequestID(r)
					if logger != nil {
						logger.Error("panic recovered",
							"request_id", reqID,
							"panic", fmt.Sprintf("%v", rec),
							"stack", string(debug.Stack()),
							"path", r.URL.Path,
						)
					}
					httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// 2. RequestID middleware
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req, reqID := httpx.EnsureRequestID(r)
			w.Header().Set("X-Request-ID", reqID)
			next.ServeHTTP(w, req)
		})
	}
}

// 3. AccessLog middleware
type accessLogContextKey string

const accessLogRecorderKey accessLogContextKey = "access_log_recorder"

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	userID     string
	role       string
}

func SetLogUser(ctx context.Context, userID, role string) {
	if rec, ok := ctx.Value(accessLogRecorderKey).(*responseRecorder); ok {
		rec.userID = userID
		rec.role = role
	}
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (rw *responseRecorder) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			ctx := context.WithValue(r.Context(), accessLogRecorderKey, rec)
			r = r.WithContext(ctx)

			next.ServeHTTP(rec, r)
			duration := time.Since(start)

			if logger != nil {
				routePattern := GetRoutePattern(r)
				if routePattern == "" {
					routePattern = "unmatched"
				}

				attrs := []any{
					"request_id", httpx.RequestID(r),
					"method", r.Method,
					"route", routePattern,
					"status", rec.statusCode,
					"latency_ms", duration.Milliseconds(),
					"user_id", rec.userID,
					"role", rec.role,
				}

				if errCode := rec.Header().Get("X-Error-Code"); errCode != "" {
					attrs = append(attrs, "error_code", errCode)
				}

				logger.Info("http request", attrs...)
			}
		})
	}
}

// 4. SecureHeaders middleware
func SecureHeaders(isLocal bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'none'")
			if !isLocal {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// 5. CORS middleware
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Idempotency-Key")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// 6. BodySizeLimit middleware (1 MB max)
const MaxBodyBytes = 1024 * 1024 // 1 MB

func BodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = MaxBodyBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request payload exceeds maximum size limit of 1 MB.", nil)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// 7. RateLimit middleware (sliding window / bucket per IP)
type ipRateLimiter struct {
	mu     sync.Mutex
	counts map[string][]time.Time
}

var globalLimiter = &ipRateLimiter{
	counts: make(map[string][]time.Time),
}

func isTrustedPeer(remoteAddr string, trustedCIDRs []*net.IPNet) bool {
	if len(trustedCIDRs) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range trustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func GetClientIP(r *http.Request, trustedCIDRs []*net.IPNet) string {
	if isTrustedPeer(r.RemoteAddr, trustedCIDRs) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func getClientIP(r *http.Request) string {
	return GetClientIP(r, nil)
}

func RateLimit(limit int, window time.Duration, trustedCIDRs ...[]*net.IPNet) func(http.Handler) http.Handler {
	var cidrs []*net.IPNet
	if len(trustedCIDRs) > 0 {
		cidrs = trustedCIDRs[0]
	}
	limiter := &ipRateLimiter{
		counts: make(map[string][]time.Time),
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip coarse rate limiting on health check and metrics endpoints
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			ip := GetClientIP(r, cidrs)
			now := time.Now()

			limiter.mu.Lock()
			cutoff := now.Add(-window)
			times := limiter.counts[ip]
			var valid []time.Time
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}

			if len(valid) == 0 {
				delete(limiter.counts, ip)
			} else {
				limiter.counts[ip] = valid
			}

			if len(valid) >= limit {
				limiter.mu.Unlock()
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				httpx.WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later.", nil)
				return
			}

			valid = append(valid, now)
			limiter.counts[ip] = valid
			limiter.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// 8. Auth middleware
func Auth(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := auth.GetSessionToken(r)
			if token != "" {
				user, err := auth.ValidateSession(r.Context(), db, token)
				if err == nil && user != nil {
					r = r.WithContext(WithUser(r.Context(), user))
					SetLogUser(r.Context(), user.ID, user.Role)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// 9. RBAC guard middleware
func RequireRoles(allowedRoles []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil {
				httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required to access this resource.", nil)
				return
			}

			if !user.IsActive {
				httpx.WriteError(w, r, http.StatusUnauthorized, "ACCOUNT_INACTIVE", "Your account has been deactivated.", nil)
				return
			}

			// Admin role has unrestricted access across all endpoints
			if user.Role == "Admin" || slices.Contains(allowedRoles, user.Role) {
				next.ServeHTTP(w, r)
				return
			}

			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to perform this operation.", nil)
		})
	}
}

func RequireAuth() func(http.Handler) http.Handler {
	return RequireRoles([]string{
		"Admin", "Fleet Manager", "Driver", "Safety Officer", "Financial Analyst",
	})
}
