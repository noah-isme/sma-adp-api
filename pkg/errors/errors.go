package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error represents a typed domain error with HTTP awareness.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
	Err     error  `json:"-"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// New creates a new Error instance.
func New(code string, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Wrap attaches context to an existing error.
func Wrap(err error, code string, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message, Err: err}
}

// Predefined errors for common scenarios.
var (
	ErrInvalidCredentials = New("INVALID_CREDENTIALS", http.StatusUnauthorized, "invalid email or password")
	ErrInactiveAccount    = New("ACCOUNT_INACTIVE", http.StatusForbidden, "account is inactive")
	ErrNotFound           = New("NOT_FOUND", http.StatusNotFound, "resource not found")
	ErrForbidden          = New("FORBIDDEN", http.StatusForbidden, "forbidden")
	ErrUnauthorized       = New("UNAUTHORIZED", http.StatusUnauthorized, "unauthorized")
	ErrConflict           = New("CONFLICT", http.StatusConflict, "conflict")
	ErrPreconditionFailed = New("PRECONDITION_FAILED", http.StatusPreconditionFailed, "precondition failed")
	ErrValidation         = New("VALIDATION_ERROR", http.StatusBadRequest, "validation failed")
	ErrInternal           = New("INTERNAL_ERROR", http.StatusInternalServerError, "internal server error")
	ErrFinalized          = New("FINALIZED", http.StatusConflict, "resource finalized")
	ErrInvalidWeights     = New("INVALID_WEIGHTS", http.StatusBadRequest, "invalid component weights")
)

// FromError normalises any error into an *Error.
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Wrap(err, ErrInternal.Code, ErrInternal.Status, ErrInternal.Message)
}

// Clone returns a copy of the error allowing for message overrides.
func Clone(err *Error, message string) *Error {
	if err == nil {
		return nil
	}
	clone := *err
	if message != "" {
		clone.Message = message
	}
	return &clone
}
