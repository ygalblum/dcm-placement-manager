package sprm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/dcm-project/placement-manager/internal/sprm"
	sprmv1alpha1 "github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SPRM Client", func() {
	var (
		ctx    context.Context
		server *httptest.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("NewClient", func() {
		It("creates a new client successfully", func() {
			client, err := sprm.NewClient("http://localhost:8082")
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})
	})

	Describe("CreateResource", func() {
		It("creates resource successfully with 201 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("POST"))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-type-instances"))
				Expect(r.URL.Query().Get("id")).To(Equal("catalog-123"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)

				id := "catalog-123"
				status := "provisioning"
				response := sprmv1alpha1.ServiceTypeInstance{
					Id:           &id,
					Status:       &status,
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 2, "memory": "4GB"},
				}
				_ = json.NewEncoder(w).Encode(response)
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			req := sprm.CreateResourceRequest{
				CatalogItemInstanceId: "catalog-123",
				Spec:                  map[string]any{"cpu": 2, "memory": "4GB"},
				ProviderName:          "test-provider",
			}

			resp, err := client.CreateResource(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.ID).To(Equal("catalog-123"))
			Expect(resp.Status).To(Equal("provisioning"))
		})

		It("handles 400 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"type": "validation-error", "title": "Invalid request"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			req := sprm.CreateResourceRequest{
				CatalogItemInstanceId: "catalog-123",
				Spec:                  map[string]any{},
				ProviderName:          "test-provider",
			}

			_, err = client.CreateResource(ctx, req)
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(400))
		})

		It("handles 500 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"type": "internal-error", "title": "Internal server error"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			req := sprm.CreateResourceRequest{
				CatalogItemInstanceId: "catalog-123",
				Spec:                  map[string]any{"cpu": 2},
				ProviderName:          "test-provider",
			}

			_, err = client.CreateResource(ctx, req)
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(500))
		})

		It("handles 409 conflict error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"type": "conflict", "title": "Resource already exists"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			req := sprm.CreateResourceRequest{
				CatalogItemInstanceId: "catalog-dup",
				Spec:                  map[string]any{"cpu": 2},
				ProviderName:          "test-provider",
			}

			_, err = client.CreateResource(ctx, req)
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(409))
		})

		It("handles 422 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"type": "provider-error", "title": "Provider validation failed"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			req := sprm.CreateResourceRequest{
				CatalogItemInstanceId: "catalog-invalid",
				Spec:                  map[string]any{"invalid": "spec"},
				ProviderName:          "test-provider",
			}

			_, err = client.CreateResource(ctx, req)
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(422))
		})
	})

	Describe("DeleteResource", func() {
		It("deletes resource successfully with 204 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("DELETE"))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-type-instances/catalog-123"))

				w.WriteHeader(http.StatusNoContent)
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeleteResource(ctx, "catalog-123")
			Expect(err).NotTo(HaveOccurred())
		})

		It("handles 404 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"type": "not-found", "title": "Resource not found"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeleteResource(ctx, "catalog-nonexistent")
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(404))
		})

		It("handles 400 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"type": "invalid-request", "title": "Invalid ID format"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeleteResource(ctx, "invalid-id")
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(400))
		})

		It("handles 500 error response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"type": "internal-error", "title": "Internal server error"}`))
			}))

			client, err := sprm.NewClient(server.URL)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeleteResource(ctx, "catalog-123")
			Expect(err).To(HaveOccurred())

			var httpErr *sprm.HTTPError
			Expect(err).To(BeAssignableToTypeOf(httpErr))
			errors.As(err, &httpErr)
			Expect(httpErr.StatusCode).To(Equal(500))
		})
	})
})
