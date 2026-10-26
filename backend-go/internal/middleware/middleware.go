package middleware

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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
// WebSocket endpoints are excluded.
func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/ws" {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
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
				routePattern := r.Pattern
				if routePattern == "" {
					routePattern = r.URL.Path
				} else {
					parts := strings.SplitN(routePattern, " ", 2)
					if len(parts) == 2 {
						routePattern = parts[1]
					}
				}

				logger.Info("http request",
					"request_id", httpx.RequestID(r),
					"method", r.Method,
					"route", routePattern,
					"status", rec.statusCode,
					"latency_ms", duration.Milliseconds(),
					"user_id", rec.userID,
					"role", rec.role,
				)
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

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := &ipRateLimiter{
		counts: make(map[string][]time.Time),
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip coarse rate limiting on health check endpoints
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			ip := getClientIP(r)
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
