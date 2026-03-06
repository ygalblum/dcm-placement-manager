package service

import (
	"errors"
	"fmt"

	"github.com/dcm-project/placement-manager/internal/policy"
	"github.com/dcm-project/placement-manager/internal/sprm"
)

// Error codes returned by service operations.
const (
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeValidation     = "VALIDATION"
	ErrCodeProviderError  = "PROVIDER_ERROR"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodePolicyError    = "POLICY_ERROR"
	ErrCodePolicyRejected = "POLICY_REJECTED"
	ErrCodePolicyConflict = "POLICY_CONFLICT"
	ErrCodeSPRMError      = "SPRM_ERROR"
)

// ServiceError represents a business logic error with a code for HTTP mapping.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// Helper functions for creating ServiceErrors

func NewNotFoundError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeNotFound,
		Message: message,
	}
}

func NewValidationError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeValidation,
		Message: message,
	}
}

func NewInternalError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeInternal,
		Message: message,
	}
}

func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeConflict,
		Message: message,
	}
}

func NewPolicyError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodePolicyError,
		Message: message,
	}
}

func NewPolicyRejectedError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodePolicyRejected,
		Message: message,
	}
}

func NewPolicyConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodePolicyConflict,
		Message: message,
	}
}

func NewSPRMError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeSPRMError,
		Message: message,
	}
}

// handlePolicyError maps policy client errors to service errors by checking
// the error type and extracting the HTTP status code.
func handlePolicyError(err error) *ServiceError {
	// Try to unwrap and get the actual error
	var httpErr *policy.HTTPError
	if errors.As(err, &httpErr) {
		// We have an HTTPError with status code
		switch httpErr.StatusCode {
		case 400:
			return NewValidationError("invalid request format for policy evaluation")
		case 406:
			return NewPolicyRejectedError("request explicitly rejected by policy")
		case 409:
			return NewPolicyConflictError("policy conflict detected")
		case 500:
			return NewPolicyError("policy engine internal error")
		default:
			return NewPolicyError(fmt.Sprintf("policy evaluation failed with status %d: %s", httpErr.StatusCode, httpErr.Body))
		}
	}

	// Default to policy error for any other error
	return NewPolicyError("policy evaluation failed: " + err.Error())
}

// handleSPRMError maps SPRM client errors to service errors by checking
// the error type and extracting the HTTP status code.
func handleSPRMError(err error) *ServiceError {
	var httpErr *sprm.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 400:
			return NewValidationError(fmt.Sprintf("invalid request format for SPRM: %s", httpErr.Body))
		case 404:
			return NewNotFoundError(fmt.Sprintf("resource not found in SPRM: %s", httpErr.Body))
		case 409:
			return NewConflictError(fmt.Sprintf("resource conflict in SPRM: %s", httpErr.Body))
		case 422:
			return &ServiceError{
				Code:    ErrCodeProviderError,
				Message: fmt.Sprintf("SPRM provider error: %s", httpErr.Body),
			}
		case 500:
			return NewSPRMError(fmt.Sprintf("SPRM internal error: %s", httpErr.Body))
		default:
			return NewSPRMError(fmt.Sprintf("SPRM request failed with status %d: %s", httpErr.StatusCode, httpErr.Body))
		}
	}

	return NewSPRMError("SPRM request failed: " + err.Error())
}
