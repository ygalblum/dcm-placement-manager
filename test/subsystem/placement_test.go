//go:build subsystem

package subsystem_test

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1alpha1 "github.com/dcm-project/placement-manager/api/v1alpha1"
)

var _ = Describe("Placement API", func() {
	BeforeEach(func() {
		resetPolicyWireMock()
		resetSPRMWireMock()
		stubPolicyEvaluateApproved("test-provider")
		stubSPRMCreateResource()
		stubSPRMDeleteResource()
	})

	Describe("CreateResource", func() {
		It("creates a resource and returns 201", func() {
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2, "memory": "4Gi"},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			Expect(resp.JSON201).NotTo(BeNil())
			Expect(resp.JSON201.Id).NotTo(BeNil())
			Expect(resp.JSON201.CatalogItemInstanceId).To(Equal(body.CatalogItemInstanceId))
			Expect(resp.JSON201.ApprovalStatus).NotTo(BeNil())
			Expect(*resp.JSON201.ApprovalStatus).To(Equal("APPROVED"))
			Expect(resp.JSON201.ProviderName).NotTo(BeNil())
			Expect(*resp.JSON201.ProviderName).To(Equal("test-provider"))

			// Verify clients are called at once
			verifyPolicyEvaluateCalled(1)
			verifySPRMCreateResourceCalled(1)
		})

		It("creates a resource with a user specified ID", func() {
			resourceID := uuid.New().String()
			params := &v1alpha1.CreateResourceParams{Id: &resourceID}
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 4},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), params, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			Expect(resp.JSON201).NotTo(BeNil())
			Expect(*resp.JSON201.Id).To(Equal(resourceID))
		})

		It("creates a resource with MODIFIED policy status", func() {
			resetPolicyWireMock()
			stubPolicyEvaluateModified("modified-provider", map[string]any{"cpu": 8, "memory": "16Gi"})

			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2, "memory": "4Gi"},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			Expect(*resp.JSON201.ApprovalStatus).To(Equal("MODIFIED"))
			Expect(*resp.JSON201.ProviderName).To(Equal("modified-provider"))
		})

		It("returns 406 when policy rejects the request", func() {
			resetPolicyWireMock()
			stubPolicyEvaluateRejected()

			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-rejected",
				Spec:                  map[string]any{"cpu": 100},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusNotAcceptable))

			verifySPRMCreateResourceCalled(0)
		})

		It("returns 500 when policy engine fails", func() {
			resetPolicyWireMock()
			stubPolicyEvaluateFailure()

			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-policy-fail",
				Spec:                  map[string]any{"cpu": 2},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusInternalServerError))

			verifySPRMCreateResourceCalled(0)
		})

		It("returns 500 and rolls back DB when SPRM fails", func() {
			resetSPRMWireMock()
			stubSPRMCreateResourceFailure()

			catalogID := "catalog-sprm-fail-" + uuid.New().String()[:8]
			body := v1alpha1.Resource{
				CatalogItemInstanceId: catalogID,
				Spec:                  map[string]any{"cpu": 2},
			}

			resp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusInternalServerError))

			verifyPolicyEvaluateCalled(1)
		})

		It("returns 409 when creating resource with duplicate ID", func() {
			resourceID := uuid.New().String()
			params := &v1alpha1.CreateResourceParams{Id: &resourceID}
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2},
			}

			resp1, err := apiClient.CreateResourceWithResponse(context.Background(), params, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp1.StatusCode()).To(Equal(http.StatusCreated))

			// Reset wiremocks for the second call
			resetPolicyWireMock()
			resetSPRMWireMock()
			stubPolicyEvaluateApproved("test-provider")
			stubSPRMCreateResource()

			body2 := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 4},
			}
			resp2, err := apiClient.CreateResourceWithResponse(context.Background(), params, body2)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.StatusCode()).To(Equal(http.StatusConflict))
		})
	})

	Describe("GetResource", func() {
		It("retrieves a created resource", func() {
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2},
			}

			createResp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))

			resourceID := *createResp.JSON201.Id

			getResp, err := apiClient.GetResourceWithResponse(context.Background(), resourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(getResp.JSON200).NotTo(BeNil())
			Expect(*getResp.JSON200.Id).To(Equal(resourceID))
			Expect(getResp.JSON200.CatalogItemInstanceId).To(Equal(body.CatalogItemInstanceId))
		})

		It("returns 404 for non-existent resource", func() {
			resp, err := apiClient.GetResourceWithResponse(context.Background(), "non-existent-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
		})
	})

	Describe("ListResources", func() {
		It("lists created resources", func() {
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-list-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2},
			}

			_, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())

			listResp, err := apiClient.ListResourcesWithResponse(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(listResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(listResp.JSON200).NotTo(BeNil())
			Expect(len(listResp.JSON200.Resources)).To(BeNumerically(">", 0))
		})

		It("paginates results", func() {
			for i := 0; i < 3; i++ {
				body := v1alpha1.Resource{
					CatalogItemInstanceId: "catalog-page-" + uuid.New().String()[:8],
					Spec:                  map[string]any{"cpu": 2},
				}

				// Reset wiremocks for each creation
				resetPolicyWireMock()
				resetSPRMWireMock()
				stubPolicyEvaluateApproved("test-provider")
				stubSPRMCreateResource()

				_, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
				Expect(err).NotTo(HaveOccurred())
			}

			pageSize := 1
			params := &v1alpha1.ListResourcesParams{MaxPageSize: &pageSize}
			listResp, err := apiClient.ListResourcesWithResponse(context.Background(), params)
			Expect(err).NotTo(HaveOccurred())
			Expect(listResp.StatusCode()).To(Equal(http.StatusOK))
			Expect(len(listResp.JSON200.Resources)).To(Equal(1))
			Expect(listResp.JSON200.NextPageToken).NotTo(BeNil())
		})
	})

	Describe("DeleteResource", func() {
		It("deletes a resource and returns 204", func() {
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-delete-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2},
			}

			createResp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))

			resourceID := *createResp.JSON201.Id

			// Reset wiremocks before delete
			resetPolicyWireMock()
			resetSPRMWireMock()
			stubSPRMDeleteResource()

			deleteResp, err := apiClient.DeleteResourceWithResponse(context.Background(), resourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteResp.StatusCode()).To(Equal(http.StatusNoContent))

			verifySPRMDeleteResourceCalled(1)

			// Verify resource is gone
			getResp, err := apiClient.GetResourceWithResponse(context.Background(), resourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.StatusCode()).To(Equal(http.StatusNotFound))
		})

		It("returns 404 for non-existent resource", func() {
			resp, err := apiClient.DeleteResourceWithResponse(context.Background(), "non-existent-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
		})

		It("preserves DB record when SPRM delete fails", func() {
			body := v1alpha1.Resource{
				CatalogItemInstanceId: "catalog-sprm-del-fail-" + uuid.New().String()[:8],
				Spec:                  map[string]any{"cpu": 2},
			}

			createResp, err := apiClient.CreateResourceWithResponse(context.Background(), nil, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(createResp.StatusCode()).To(Equal(http.StatusCreated))

			resourceID := *createResp.JSON201.Id

			// Reset wiremocks and stub SPRM failure
			resetPolicyWireMock()
			resetSPRMWireMock()
			stubSPRMDeleteResourceFailure()

			deleteResp, err := apiClient.DeleteResourceWithResponse(context.Background(), resourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleteResp.StatusCode()).To(Equal(http.StatusInternalServerError))

			// Verify the resource is still in the DB
			getResp, err := apiClient.GetResourceWithResponse(context.Background(), resourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(getResp.StatusCode()).To(Equal(http.StatusOK))
		})
	})
})
