package handlers

import (
	"errors"

	"github.com/dcm-project/placement-manager/internal/api/server"
	"github.com/dcm-project/placement-manager/internal/service"
)

// newError creates an RFC 7807 compliant error response.
func newError(errType, title, detail string, status int) server.Error {
	return server.Error{
		Type:   errType,
		Title:  title,
		Detail: &detail,
		Status: &status,
	}
}

// handleListResourcesError converts a service error to a ListResources response.
func handleListResourcesError(err error) server.ListResourcesResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) && svcErr.Code == service.ErrCodeValidation {
		return server.ListResources400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
	}
	return server.ListResourcesdefaultApplicationProblemPlusJSONResponse{
		Body:       newError("list-error", "Failed to list resources", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleCreateResourceError converts a service error to a CreateResource response.
func handleCreateResourceError(err error) server.CreateResourceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.CreateResource400ApplicationProblemPlusJSONResponse(newError("validation-error", "Validation failed", svcErr.Message, 400))
		case service.ErrCodePolicyRejected:
			return server.CreateResource406ApplicationProblemPlusJSONResponse(newError("policy-rejected", "Policy rejected request", svcErr.Message, 406))
		case service.ErrCodeConflict:
			return server.CreateResource409ApplicationProblemPlusJSONResponse(newError("resource-conflict", "Resource already exists", svcErr.Message, 409))
		case service.ErrCodePolicyConflict:
			return server.CreateResource409ApplicationProblemPlusJSONResponse(newError("policy-conflict", "Policy conflict", svcErr.Message, 409))
		case service.ErrCodeProviderError:
			return server.CreateResource422ApplicationProblemPlusJSONResponse(newError("provider-error", "Provider error", svcErr.Message, 422))
		case service.ErrCodeInternal, service.ErrCodePolicyError, service.ErrCodePolicyInternalError, service.ErrCodeSPRMError:
			return server.CreateResourcedefaultApplicationProblemPlusJSONResponse{
				Body:       newError("internal-error", "Internal error", svcErr.Message, 500),
				StatusCode: 500,
			}
		}
	}
	return server.CreateResourcedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("create-error", "Failed to create resource", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleGetResourceError converts a service error to a GetResource response.
func handleGetResourceError(err error) server.GetResourceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.GetResource400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
		case service.ErrCodeNotFound:
			return server.GetResource404ApplicationProblemPlusJSONResponse(newError("not-found", "Resource not found", svcErr.Message, 404))
		}
	}
	return server.GetResourcedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("get-error", "Failed to get resource", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleDeleteResourceError converts a service error to a DeleteResource response.
func handleDeleteResourceError(err error) server.DeleteResourceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.DeleteResource400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
		case service.ErrCodeNotFound:
			return server.DeleteResource404ApplicationProblemPlusJSONResponse(newError("not-found", "Resource not found", svcErr.Message, 404))
		case service.ErrCodeProviderError:
			return server.DeleteResource422ApplicationProblemPlusJSONResponse(newError("provider-error", "Provider error", svcErr.Message, 422))
		case service.ErrCodeInternal, service.ErrCodeSPRMError:
			return server.DeleteResourcedefaultApplicationProblemPlusJSONResponse{
				Body:       newError("internal-error", "Internal error", svcErr.Message, 500),
				StatusCode: 500,
			}
		}
	}
	return server.DeleteResourcedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("delete-error", "Failed to delete resource", err.Error(), 500),
		StatusCode: 500,
	}
}
