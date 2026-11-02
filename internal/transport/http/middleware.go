package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/auth"
	"github.com/google/uuid"
)

// Context keys for request-scoped values.
type contextKey string

const (
	ctxKeyRequestID contextKey = "request_id"
	ctxKeyUserID    contextKey = "user_id"
	maxBodyBytes    int64      = 2 << 20 // 2 MiB payload limit
)

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// UserIDFromContext extracts the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyUserID).(string)
	return id
}

// RequestID generates a UUID for each request, adds it to the context, and
// sets the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" || len(id) > 64 {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MaxBodySize limits incoming HTTP request bodies to prevent memory exhaustion.
func MaxBodySize(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wrote {
		rw.status = code
		rw.wrote = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.status = http.StatusOK
		rw.wrote = true
	}
	return rw.ResponseWriter.Write(b)
}

// StructuredLogger logs each request using slog without logging sensitive headers or bodies.
func StructuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			if logger != nil {
				logger.Info("request completed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", wrapped.status,
					"duration_ms", time.Since(start).Milliseconds(),
					"request_id", RequestIDFromContext(r.Context()),
					"remote_addr", r.RemoteAddr,
				)
			}
		})
	}
}

// AuthMiddleware validates the Bearer JWT from the Authorization header and attaches
// the authenticated user ID to the request context.
func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Invalid Authorization header format")
				return
			}

			token := strings.TrimSpace(parts[1])
			if token == "" {
				RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Empty bearer token")
				return
			}

			claims, err := auth.ValidateToken(token, jwtSecret)
			if err != nil {
				msg := "Invalid authentication token"
				if errors.Is(err, auth.ErrExpiredToken) {
					msg = "Authentication token has expired"
				}
				RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, msg)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
