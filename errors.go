package indigo

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents a non-2xx response from the WebARENA Indigo API.
type APIError struct {
	StatusCode int
	Body       string
	RequestID  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("indigo: API error %d: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is a 401 API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}
