package orca

import (
	"encoding/json"
	"errors"
	"fmt"
)

// APIError represents an error response from the Orca API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("orca api error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound reports whether the error is a 404 Not Found response.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

// IsUnauthorized reports whether the error is a 401 Unauthorized response.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 401
}

// IsForbidden reports whether the error is a 403 Forbidden response.
func IsForbidden(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 403
}

// IsValidationError reports whether the error is a 422 Validation Error response.
func IsValidationError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 422
}

func parseAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode}

	var envelope struct {
		Detail json.RawMessage `json:"detail"`
		Reason string          `json:"reason"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Reason != "" {
			apiErr.Message = envelope.Reason
			return apiErr
		}
		if envelope.Detail != nil {
			var s string
			if json.Unmarshal(envelope.Detail, &s) == nil {
				apiErr.Message = s
				return apiErr
			}
			apiErr.Message = string(envelope.Detail)
			return apiErr
		}
	}

	if len(body) > 0 {
		apiErr.Message = string(body)
	} else {
		apiErr.Message = fmt.Sprintf("HTTP %d", statusCode)
	}
	return apiErr
}
