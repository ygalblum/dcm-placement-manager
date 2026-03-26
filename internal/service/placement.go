// Package service implements the core business logic for resource placement.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/dcm-project/placement-manager/internal/api/server"
	"github.com/dcm-project/placement-manager/internal/logging"
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
	log := logging.FromContext(ctx)

	// Get or Generate ID
	resourceIDStr := getOrGenerateStringId(queryId)

	// Generate path
	path := fmt.Sprintf("resources/%s", resourceIDStr)

	log.Debug("Creating resource",
		"resource_id", resourceIDStr,
		"catalog_item_instance_id", req.CatalogItemInstanceId,
	)

	// Validate request with policy engine

	// Build policy payload
	policyRequest := policy.EvaluateRequest{
		Spec: req.Spec,
	}

	// Evaluate spec
	log.Debug("Evaluating policy", "resource_id", resourceIDStr)
	policyResponse, err := s.policy.Evaluate(ctx, policyRequest)
	if err != nil {
		log.Error("Policy evaluation failed", "resource_id", resourceIDStr, "error", err)
		return nil, handlePolicyError(err)
	}

	if policyResponse.SelectedProvider == "" {
		log.Error("Policy response missing selected provider",
			"resource_id", resourceIDStr,
			"status", policyResponse.Status,
		)
		return nil, NewPolicyInternalError("policy response missing selected provider")
	}

	// Extract approvalStatus and providerName from policy response
	approvalStatus := policyResponse.Status
	providerName := policyResponse.SelectedProvider

	// Update request with status and selected provider
	req.ApprovalStatus = &approvalStatus
	req.ProviderName = &providerName

	// Convert API resource to store model
	requestModel := resourceToStoreModel(req, resourceIDStr, path)

	// Create resource in store
	created, err := s.store.Resource().Create(ctx, requestModel)
	if err != nil {
		if errors.Is(err, store.ErrResourceIdExist) {
			log.Warn("Duplicate resource ID", "resource_id", resourceIDStr)
			return nil, NewConflictError(fmt.Sprintf("resource with id %s already exists", resourceIDStr))
		}
		log.Error("Failed to create resource in store", "resource_id", resourceIDStr, "error", err)
		return nil, NewInternalError(fmt.Sprintf("failed to create database record for resource %s: %v", resourceIDStr, err))
	}

	log.Debug("Resource persisted in store", "resource_id", resourceIDStr)

	// Send request to SP Resource Manager
	sprmRequest := sprm.CreateResourceRequest{
		CatalogItemInstanceId: created.CatalogItemInstanceId,
		Spec:                  policyResponse.EvaluatedSpec,
		ProviderName:          providerName,
	}

	log.Debug("Provisioning resource via SPRM",
		"resource_id", resourceIDStr,
		"catalog_item_instance_id", created.CatalogItemInstanceId,
	)

	sprmResponse, err := s.sprm.CreateResource(ctx, sprmRequest)
	if err != nil {
		// SPRM call failed, rollback the database record
		log.Error("SPRM provisioning failed, rolling back", "resource_id", resourceIDStr, "error", err)
		if delErr := s.store.Resource().Delete(ctx, created.ID); delErr != nil {
			log.Error("Failed to rollback resource after SPRM error",
				"resource_id", created.ID,
				"db_error", delErr,
				"sprm_error", err,
			)
		}
		return nil, handleSPRMError(err)
	}

	log.Info("Resource created successfully",
		"resource_id", created.ID,
		"catalog_item_instance_id", created.CatalogItemInstanceId,
		"provider", providerName,
		"approval_status", approvalStatus,
		"sprm_status", sprmResponse.Status,
	)
	return storeModelToResource(created), nil
}

// GetResource retrieves a placement request by ID.
func (s *PlacementService) GetResource(ctx context.Context, requestID string) (*server.Resource, error) {
	log := logging.FromContext(ctx)
	log.Debug("Getting resource", "resource_id", requestID)

	request, err := s.store.Resource().Get(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return nil, NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		log.Error("Failed to get resource from store", "resource_id", requestID, "error", err)
		return nil, NewInternalError(fmt.Sprintf("failed to retrieve resource: %v", err))
	}

	log.Debug("Resource retrieved", "resource_id", requestID)
	return storeModelToResource(request), nil
}

// ListResources returns placement requests with optional filtering and pagination.
func (s *PlacementService) ListResources(ctx context.Context, providerName *string, maxPageSize *int, pageToken *string) (*server.ResourceList, error) {
	log := logging.FromContext(ctx)
	log.Debug("Listing resources",
		"provider_filter", providerName,
		"page_size", maxPageSize,
	)

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
		log.Error("Failed to list resources from store", "error", err)
		return nil, NewInternalError(fmt.Sprintf("failed to list resources: %v", err))
	}

	// Convert to API types
	resources := make([]server.Resource, len(result.Resources))
	for i, resource := range result.Resources {
		resources[i] = *storeModelToResource(&resource)
	}

	log.Debug("Resources listed",
		"count", len(resources),
		"has_next_page", result.NextPageToken != nil,
	)
	return &server.ResourceList{
		Resources:     resources,
		NextPageToken: result.NextPageToken,
	}, nil
}

// DeleteResource removes a placement request by ID.
func (s *PlacementService) DeleteResource(ctx context.Context, requestID string) error {
	log := logging.FromContext(ctx)
	log.Debug("Deleting resource", "resource_id", requestID)

	// First, get the resource to obtain the CatalogItemInstanceId
	resource, err := s.store.Resource().Get(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		log.Error("Failed to get resource for deletion", "resource_id", requestID, "error", err)
		return NewInternalError(fmt.Sprintf("failed to retrieve resource for deletion: %v", err))
	}

	// Delete it from the SPRM first before deleting from the database
	log.Debug("Deleting resource from SPRM",
		"resource_id", requestID,
		"catalog_item_instance_id", resource.CatalogItemInstanceId,
	)

	err = s.sprm.DeleteResource(ctx, resource.CatalogItemInstanceId)
	if err != nil {
		log.Error("SPRM deletion failed, preserving DB record", "resource_id", requestID, "error", err)
		return handleSPRMError(err)
	}

	// Delete record from the database
	err = s.store.Resource().Delete(ctx, requestID)
	if err != nil {
		if errors.Is(err, store.ErrResourceNotFound) {
			return NewNotFoundError(fmt.Sprintf("resource %s not found", requestID))
		}
		log.Error("Failed to delete resource from store", "resource_id", requestID, "error", err)
		return NewInternalError(fmt.Sprintf("failed to delete database record for resource %s: %v", requestID, err))
	}

	log.Info("Resource deleted successfully",
		"resource_id", requestID,
		"catalog_item_instance_id", resource.CatalogItemInstanceId,
	)
	return nil
}

func getOrGenerateStringId(id *string) string {
	if id != nil && *id != "" {
		return *id
	}
	// Generate UUID if not provided
	return uuid.New().String()
}
