package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/dcm-project/placement-manager/internal/api/server"
	"github.com/dcm-project/placement-manager/internal/policy"
	"github.com/dcm-project/placement-manager/internal/sprm"
	"github.com/dcm-project/placement-manager/internal/store"
	"github.com/google/uuid"
)

// PlacementService handles business logic for placement request management.
type PlacementService struct {
	store  store.Store
	policy policy.Client
	sprm   sprm.Client
}

// NewPlacementService creates a new PlacementService with the given store, policy client, and SPRM client.
func NewPlacementService(store store.Store, policyClient policy.Client, sprmClient sprm.Client) *PlacementService {
	return &PlacementService{
		store:  store,
		policy: policyClient,
		sprm:   sprmClient,
	}
}

// CreateResource creates a new placement request.
func (s *PlacementService) CreateResource(ctx context.Context, req *server.Resource, queryId *string) (*server.Resource, error) {
	// Get or Generate ID
	resourceIDStr := getOrGenerateStringId(queryId)

	// Generate path
	path := fmt.Sprintf("resources/%s", resourceIDStr)

	// Validate request with policy engine

	// Build policy payload
	policyRequest := policy.EvaluateRequest{
		Spec: req.Spec,
	}

	// Evaluate spec
	policyResponse, err := s.policy.Evaluate(ctx, policyRequest)
	if err != nil {
		return nil, handlePolicyError(err)
	}

	// Extract approvalStatus and providerName from policy response
	approvalStatus := policyResponse.Status
	providerName := policyResponse.SelectedProvider

	// Update request with status and selected provider
	req.ApprovalStatus = &approvalStatus
	req.ProviderName = &providerName

	// Convert API resource to store model
	requestModel := resourceToStoreModel(req, resourceIDStr, path)
	requestModel.ProviderName = &providerName
	requestModel.ApprovalStatus = &approvalStatus

	// Create resource in store
	created, err := s.store.Resource().Create(ctx, requestModel)
	if err != nil {
		if errors.Is(err, store.ErrResourceIdExist) {
			return nil, NewConflictError(fmt.Sprintf("resource with id %s already exists", resourceIDStr))
		}
		return nil, NewInternalError(fmt.Sprintf("failed to create database record for resource %s: %v", resourceIDStr, err))
	}

	// Send request to SP Resource Manager
	sprmRequest := sprm.CreateResourceRequest{
		CatalogItemInstanceId: created.CatalogItemInstanceId,
		Spec:                  policyResponse.EvaluatedSpec,
		ProviderName:          providerName,
	}

	sprmResponse, err := s.sprm.CreateResource(ctx, sprmRequest)
	if err != nil {
		// SPRM call failed, rollback the database record
		if err := s.store.Resource().Delete(ctx, created.ID); err != nil {
			log.Printf("Failed to rollback resource %s after SPRM error: %v", created.ID, err)
		}
		return nil, handleSPRMError(err)
	}

	log.Printf("Successfully created resource: %s (catalog_item_instance_id: %s, provider: %s, sprm_status: %s)",
		created.ID, created.CatalogItemInstanceId, providerName, sprmResponse.Status)
	return storeModelToResource(created), nil
}

// GetResource retrieves a placement request by ID.
func (s *PlacementService) GetResource(ctx context.Context, requestID string) (*server.Resource, error) {
	request, err := s.store.Resource().Get(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return nil, NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		return nil, NewInternalError(fmt.Sprintf("failed to retrieve resource: %v", err))
	}

	return storeModelToResource(request), nil
}

// ListResources returns placement requests with optional filtering and pagination.
func (s *PlacementService) ListResources(ctx context.Context, providerName *string, maxPageSize *int, pageToken *string) (*server.ResourceList, error) {
	opts := &store.ResourceListOptions{
		ProviderName: providerName,
	}

	// Apply max page size
	if maxPageSize != nil {
		if *maxPageSize > 0 && *maxPageSize <= 100 {
			opts.PageSize = *maxPageSize
		} else {
			return nil, NewValidationError("page size must be between 1 and 100")
		}
	}

	// Apply page token
	if pageToken != nil && *pageToken != "" {
		opts.PageToken = pageToken
	}

	// Get resources from store
	result, err := s.store.Resource().List(ctx, opts)
	if err != nil {
		return nil, NewInternalError(fmt.Sprintf("failed to list resources: %v", err))
	}

	// Convert to API types
	resources := make([]server.Resource, len(result.Resources))
	for i, resource := range result.Resources {
		resources[i] = *storeModelToResource(&resource)
	}

	return &server.ResourceList{
		Resources:     resources,
		NextPageToken: result.NextPageToken,
	}, nil
}

// DeleteResource removes a placement request by ID.
func (s *PlacementService) DeleteResource(ctx context.Context, requestID string) error {
	// First, get the resource to obtain the CatalogItemInstanceId
	resource, err := s.store.Resource().Get(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		return NewInternalError(fmt.Sprintf("failed to retrieve resource for deletion: %v", err))
	}

	// Delete it from the SPRM first before deleting from the database
	err = s.sprm.DeleteResource(ctx, resource.CatalogItemInstanceId)
	if err != nil {
		return handleSPRMError(err)
	}

	// Delete record from the database
	err = s.store.Resource().Delete(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		return NewInternalError(fmt.Sprintf("failed to delete database record for resource %s: %v", requestID, err))
	}

	log.Printf("Deleted resource from SPRM and DB: %s (catalog_item_instance_id: %s)", requestID, resource.CatalogItemInstanceId)
	return nil
}

func getOrGenerateStringId(id *string) string {
	if id != nil && *id != "" {
		return *id
	}
	// Generate UUID if not provided
	return uuid.New().String()
}
