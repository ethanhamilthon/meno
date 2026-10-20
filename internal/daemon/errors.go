package daemon

import (
	"errors"
	"meno/internal/proto"
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
	case proto.CodeBadRequest:
		return http.StatusBadRequest
	case proto.CodeNotFound:
		return http.StatusNotFound
	case proto.CodeForbidden:
		return http.StatusForbidden
	case proto.CodeConflict:
		return http.StatusConflict
	case proto.CodeProtocolMismatch:
		return http.StatusUpgradeRequired
	case proto.CodeDaemonStopping:
		return http.StatusServiceUnavailable
	case proto.CodeNotImplemented:
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
	return &APIError{Code: proto.CodeInternal, Message: err.Error(), Status: http.StatusInternalServerError}
}
