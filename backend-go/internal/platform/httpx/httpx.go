package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

type ErrorDetail struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Details   map[string]any `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

func RequestID(r *http.Request) string {
	if v, ok := r.Context().Value(RequestIDKey).(string); ok && v != "" {
		return v
	}
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

func EnsureRequestID(r *http.Request) (*http.Request, string) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = uuid.NewString()
	}
	return r.WithContext(WithRequestID(r.Context(), reqID)), reqID
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	reqID := RequestID(r)
	if reqID == "" {
		reqID = uuid.NewString()
	}

	envelope := ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: reqID,
			Details:   details,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope)
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request payload exceeds maximum allowed size.", nil)
			return err
		}
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
		return err
	}
	return nil
}

// MapDBError converts PostgreSQL errors to appropriate HTTP status and error codes.
func MapDBError(err error, logger *slog.Logger) (int, string, string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return http.StatusConflict, "CONFLICT", "A resource with this identifier or unique attribute already exists."
		case "23503": // foreign_key_violation
			return http.StatusConflict, "FOREIGN_KEY_VIOLATION", "Referenced entity does not exist or cannot be removed due to existing relations."
		case "23514": // check_violation
			return http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Value violates schema check constraint."
		case "22P02": // invalid_text_representation
			return http.StatusBadRequest, "MALFORMED_INPUT", "Malformed data format for input type."
		}
	}
	if logger != nil {
		logger.Error("unhandled internal database error", "error", err)
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected internal server error occurred."
}
