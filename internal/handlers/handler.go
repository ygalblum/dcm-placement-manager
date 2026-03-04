package handlers

import (
	"context"

	"github.com/dcm-project/placement-manager/internal/api/server"
	"github.com/dcm-project/placement-manager/internal/service"
)

// Handler implements the generated StrictServerInterface for the Placement API.
type Handler struct {
	placementService *service.PlacementService
}

// NewHandler creates a new Handler with the given placement service.
func NewHandler(placementService *service.PlacementService) *Handler {
	return &Handler{placementService: placementService}
}

// Ensure Handler implements StrictServerInterface
var _ server.StrictServerInterface = (*Handler)(nil)

// GetHealth returns the health status of the service.
func (h *Handler) GetHealth(ctx context.Context, request server.GetHealthRequestObject) (server.GetHealthResponseObject, error) {
	status := "ok"
	path := "health"
	return server.GetHealth200JSONResponse{Status: status, Path: &path}, nil
}

// ListResources returns a paginated list of resources.
func (h *Handler) ListResources(ctx context.Context, request server.ListResourcesRequestObject) (server.ListResourcesResponseObject, error) {
	result, err := h.placementService.ListResources(
		ctx,
		request.Params.Provider,
		request.Params.MaxPageSize,
		request.Params.PageToken,
	)
	if err != nil {
		return handleListResourcesError(err), nil
	}

	return server.ListResources200JSONResponse{
		Resources:     result.Resources,
		NextPageToken: result.NextPageToken,
	}, nil
}

// CreateResource creates a new resource.
func (h *Handler) CreateResource(ctx context.Context, request server.CreateResourceRequestObject) (server.CreateResourceResponseObject, error) {
	result, err := h.placementService.CreateResource(ctx, request.Body, request.Params.Id)
	if err != nil {
		return handleCreateResourceError(err), nil
	}

	return server.CreateResource201JSONResponse(*result), nil
}

// GetResource retrieves a resource by ID.
func (h *Handler) GetResource(ctx context.Context, request server.GetResourceRequestObject) (server.GetResourceResponseObject, error) {
	result, err := h.placementService.GetResource(ctx, request.ResourceId)
	if err != nil {
		return handleGetResourceError(err), nil
	}
	return server.GetResource200JSONResponse(*result), nil
}

// DeleteResource deletes a resource by ID.
func (h *Handler) DeleteResource(ctx context.Context, request server.DeleteResourceRequestObject) (server.DeleteResourceResponseObject, error) {
	err := h.placementService.DeleteResource(ctx, request.ResourceId)
	if err != nil {
		return handleDeleteResourceError(err), nil
	}

	return server.DeleteResource204Response{}, nil
}
