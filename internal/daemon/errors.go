package daemon

import (
	"errors"
	"meno/internal/api"
	"net/http"
)

// APIError is a backend or routing error with an HTTP status and wire code.
type APIError struct {
	Code    string
	Message string
	Status  int
}

func (e *APIError) Error() string { return e.Message }
func statusFor(code string) int {
	switch code {
	case api.CodeBadRequest:
		return http.StatusBadRequest
	case api.CodeNotFound:
		return http.StatusNotFound
	case api.CodeForbidden:
		return http.StatusForbidden
	case api.CodeConflict:
		return http.StatusConflict
	case api.CodeNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
func apiError(err error) *APIError {
	var e *APIError
	if errors.As(err, &e) {
		if e.Status == 0 {
			e.Status = statusFor(e.Code)
		}
		return e
	}
	return &APIError{Code: api.CodeInternal, Message: err.Error(), Status: http.StatusInternalServerError}
}
