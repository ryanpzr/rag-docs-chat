// Package apperrors defines errors that carry an HTTP status code, so
// usecases can decide how a failure is reported without knowing about HTTP
// handlers.
package apperrors

import "net/http"

type AppError struct {
	Code    int
	Message string
	// Err is the underlying cause. It is logged but never sent to clients.
	Err error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func NewBadRequest(msg string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: msg}
}

func NewPayloadTooLarge(msg string) *AppError {
	return &AppError{Code: http.StatusRequestEntityTooLarge, Message: msg}
}

func NewUnsupportedMediaType(msg string) *AppError {
	return &AppError{Code: http.StatusUnsupportedMediaType, Message: msg}
}

func NewInternal(msg string, err error) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: msg, Err: err}
}

func NewBadGateway(msg string, err error) *AppError {
	return &AppError{Code: http.StatusBadGateway, Message: msg, Err: err}
}
