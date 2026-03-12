package sprm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/dcm-project/placement-manager/internal/httputil"
	sprmv1alpha1 "github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	sprmclient "github.com/dcm-project/service-provider-manager/pkg/client/resource_manager"
)

// CreateResourceRequest is the request body for creating a resource in SPRM
type CreateResourceRequest struct {
	CatalogItemInstanceId string         `json:"catalog_item_instance_id"`
	Spec                  map[string]any `json:"spec"`
	ProviderName          string         `json:"provider_name"`
}

// CreateResourceResponse is the response from creating a resource
type CreateResourceResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Client defines the interface for interacting with the Service Provider Resource Manager
type Client interface {
	CreateResource(ctx context.Context, req CreateResourceRequest) (*CreateResourceResponse, error)
	DeleteResource(ctx context.Context, catalogItemInstanceId string) error
}

// HTTPError represents an HTTP error from the SPRM
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("sprm returned status %d: %s", e.StatusCode, e.Body)
}

type client struct {
	sprm      *sprmclient.ClientWithResponses
	retryOpts []backoff.RetryOption
}

// NewClient creates a new Service Provider Resource Manager client
func NewClient(baseURL string, timeout time.Duration, opts ...sprmclient.ClientOption) (Client, error) {
	httpClient := &http.Client{Timeout: timeout}
	opts = append([]sprmclient.ClientOption{sprmclient.WithHTTPClient(httpClient)}, opts...)

	sprm, err := sprmclient.NewClientWithResponses(baseURL+"/api/v1alpha1", opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create sprm client: %w", err)
	}

	return &client{
		sprm:      sprm,
		retryOpts: httputil.DefaultRetryOpts(),
	}, nil
}

// CreateResource sends a resource creation request to the appropriate service provider
func (c *client) CreateResource(ctx context.Context, req CreateResourceRequest) (*CreateResourceResponse, error) {
	// Build the request body
	body := sprmv1alpha1.ServiceTypeInstance{
		ProviderName: req.ProviderName,
		Spec:         req.Spec,
	}

	// Use CatalogItemInstanceId as the query parameter id
	params := &sprmv1alpha1.CreateInstanceParams{
		Id: &req.CatalogItemInstanceId,
	}

	// Call the SP Resource Manager API
	operation := func() (*CreateResourceResponse, error) {
		resp, err := c.sprm.CreateInstanceWithResponse(ctx, params, body)
		if err != nil {
			return nil, fmt.Errorf("failed to call sprm: %w", err)
		}

		if resp.JSON201 == nil {
			httpErr := &HTTPError{
				StatusCode: resp.StatusCode(),
				Body:       string(resp.Body),
			}
			if httputil.IsPermanentHTTPError(resp.StatusCode()) {
				return nil, backoff.Permanent(httpErr)
			}
			return nil, httpErr
		}

		return mapCreateInstanceResponse(resp.JSON201), nil
	}

	return backoff.Retry(ctx, operation, c.retryOpts...)
}

func mapCreateInstanceResponse(instance *sprmv1alpha1.ServiceTypeInstance) *CreateResourceResponse {
	response := &CreateResourceResponse{}

	if instance.Id != nil {
		response.ID = *instance.Id
	}

	if instance.Status != nil {
		response.Status = *instance.Status
	}

	return response
}

// DeleteResource deletes a resource from the service provider
func (c *client) DeleteResource(ctx context.Context, catalogItemInstanceId string) error {
	// Call the SPRM API to delete the instance
	operation := func() (any, error) {
		resp, err := c.sprm.DeleteInstanceWithResponse(ctx, catalogItemInstanceId)
		if err != nil {
			return nil, fmt.Errorf("failed to call sprm delete: %w", err)
		}

		if resp.StatusCode() == 204 {
			return nil, nil
		}

		httpErr := &HTTPError{
			StatusCode: resp.StatusCode(),
			Body:       string(resp.Body),
		}

		if httputil.IsPermanentHTTPError(resp.StatusCode()) {
			return nil, backoff.Permanent(httpErr)
		}
		return nil, httpErr
	}

	_, err := backoff.Retry(ctx, operation, c.retryOpts...)
	return err
}
