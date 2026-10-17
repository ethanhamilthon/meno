package llm

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPError is a non-2xx answer of the provider.
type HTTPError struct {
	Status     int
	Body       string
	RetryAfter time.Duration // from the Retry-After header, 0 when absent
}

func (e *HTTPError) Error() string {
	return "http " + strconv.Itoa(e.Status) + ": " + e.Body
}

// permanentError marks failures that no retry can fix (a request that could not
// even be built).
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Retryable reports whether sending the same request again can succeed, and the
// delay the provider asked for (0 if none). Network failures, dropped streams,
// provider errors inside a stream, 408, 425, 429 and 5xx are retryable; other
// 4xx answers (bad key, bad request, unknown model) are not.
func Retryable(err error) (ok bool, after time.Duration) {
	if err == nil || errors.Is(err, context.Canceled) {
		return false, 0
	}
	var perm permanentError
	if errors.As(err, &perm) {
		return false, 0
	}
	var he *HTTPError
	if errors.As(err, &he) {
		switch {
		case he.Status == http.StatusRequestTimeout, he.Status == http.StatusTooEarly, he.Status == http.StatusTooManyRequests, he.Status >= 500:
			return true, he.RetryAfter
		}
		return false, 0
	}
	return true, 0
}

// parseRetryAfter reads delta-seconds or an HTTP date.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
