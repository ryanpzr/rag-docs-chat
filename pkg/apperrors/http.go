package apperrors

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// HandleError writes err as a JSON error response.
// Client errors (4xx) expose their message. Server errors (5xx) and unknown
// errors are logged with their cause; the client only gets the message,
// never database, driver or provider details.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = NewInternal("internal server error", err)
	}
	if appErr.Code >= http.StatusInternalServerError {
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	}
	WriteJSON(w, appErr.Code, map[string]string{"error": appErr.Message})
}
