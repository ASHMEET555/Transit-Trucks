package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
	if v := r.Header.Get("X-Request-ID"); v != "" && isSafeRequestID(v) {
		return v
	}
	return ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

func isSafeRequestID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

func EnsureRequestID(r *http.Request) (*http.Request, string) {
	reqID := r.Header.Get("X-Request-ID")
	if !isSafeRequestID(reqID) {
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

	w.Header().Set("X-Error-Code", code)
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
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request payload exceeds maximum allowed size.", nil)
			return err
		}
		var unmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &unmarshalTypeError) {
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid field type.", map[string]any{
				"field": unmarshalTypeError.Field,
			})
			return err
		}
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			fieldName = strings.Trim(fieldName, `"`)
			WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Unknown field: "+fieldName, map[string]any{
				"field": fieldName,
			})
			return err
		}
		WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
		return err
	}
	return nil
}

// MapDBError converts PostgreSQL errors to appropriate HTTP status and error codes.
func MapDBError(err error, logger *slog.Logger) (int, string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusServiceUnavailable, "REQUEST_TIMEOUT", "Request timed out."
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "57014" { // query_canceled / statement_timeout
			return http.StatusGatewayTimeout, "DB_TIMEOUT", "Database query execution exceeded statement timeout."
		}
		switch pgErr.Code {
		case "23505": // unique_violation
			switch pgErr.ConstraintName {
			case "idx_trips_vehicle_dispatched":
				return http.StatusConflict, "TRIP_VEHICLE_UNAVAILABLE", "Vehicle is actively dispatched on another trip."
			case "idx_trips_driver_dispatched":
				return http.StatusConflict, "TRIP_DRIVER_UNAVAILABLE", "Driver is actively dispatched on another trip."
			case "idx_maintenance_vehicle_open":
				return http.StatusConflict, "VEHICLE_IN_MAINTENANCE", "Vehicle currently has an open maintenance log."
			case "idempotency_keys_pkey":
				return http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "A request with this idempotency key is currently in progress."
			case "vehicles_pkey":
				return http.StatusConflict, "VEHICLE_EXISTS", "A vehicle with this registration number already exists."
			case "drivers_pkey":
				return http.StatusConflict, "DRIVER_EXISTS", "A driver with this license number already exists."
			case "destinations_pkey":
				return http.StatusConflict, "DESTINATION_EXISTS", "A destination with this name already exists."
			default:
				if strings.Contains(pgErr.TableName, "vehicle") {
					return http.StatusConflict, "VEHICLE_EXISTS", "A vehicle with this registration number already exists."
				}
				if strings.Contains(pgErr.TableName, "driver") {
					return http.StatusConflict, "DRIVER_EXISTS", "A driver with this license number already exists."
				}
				if strings.Contains(pgErr.TableName, "destination") {
					return http.StatusConflict, "DESTINATION_EXISTS", "A destination with this name already exists."
				}
				return http.StatusConflict, "CONFLICT", "A resource with this identifier or unique attribute already exists."
			}
		case "23503": // foreign_key_violation
			return http.StatusConflict, "FOREIGN_KEY_VIOLATION", "Referenced entity does not exist or cannot be removed due to existing relations."
		case "23514": // check_violation
			return http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Value violates schema check constraint."
		case "22P02": // invalid_text_representation
			return http.StatusBadRequest, "MALFORMED_INPUT", "Malformed data format for input type."
		default:
			if strings.HasPrefix(pgErr.Code, "23") {
				return http.StatusConflict, "CONFLICT", "Database constraint violation."
			}
		}
	}
	if logger != nil {
		logger.Error("unhandled internal database error", "error", err)
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR", "internal error"
}
