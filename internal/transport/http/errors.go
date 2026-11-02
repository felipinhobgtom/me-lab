package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Standard error codes returned by the API.
const (
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeValidation      = "VALIDATION_ERROR"
	ErrCodeInternal        = "INTERNAL_ERROR"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeBadRequest      = "BAD_REQUEST"
)

// APIError is the error envelope returned to clients.
type APIError struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the structured error information.
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Details   map[string]any `json:"details,omitempty"`
}

// RespondJSON writes a JSON response with the given status code.
func RespondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
	}
}

// RespondError writes a structured error response.
func RespondError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	reqID, _ := r.Context().Value(ctxKeyRequestID).(string)

	apiErr := APIError{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: reqID,
		},
	}

	RespondJSON(w, status, apiErr)
}

// RespondValidationError writes a 422 response with field-level validation details.
func RespondValidationError(w http.ResponseWriter, r *http.Request, details map[string]any) {
	reqID, _ := r.Context().Value(ctxKeyRequestID).(string)

	apiErr := APIError{
		Error: ErrorBody{
			Code:      ErrCodeValidation,
			Message:   "Validation failed",
			RequestID: reqID,
			Details:   details,
		},
	}

	RespondJSON(w, http.StatusUnprocessableEntity, apiErr)
}
