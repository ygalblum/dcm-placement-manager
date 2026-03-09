package policy_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/dcm-project/placement-manager/internal/policy"
	enginev1alpha1 "github.com/dcm-project/policy-manager/api/v1alpha1/engine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy Client", func() {
	var (
		server     *httptest.Server
		client     policy.Client
		ctx        context.Context
		statusCode int
		response   interface{}
	)

	BeforeEach(func() {
		ctx = context.Background()
		statusCode = http.StatusOK
		response = enginev1alpha1.EvaluateResponse{
			Status:           enginev1alpha1.APPROVED,
			SelectedProvider: "aws",
			EvaluatedServiceInstance: enginev1alpha1.ServiceInstance{
				Spec: map[string]interface{}{
					"cpu":    "2",
					"memory": "4Gi",
				},
			},
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(response)
		}))

		var err error
		client, err = policy.NewClient(server.URL)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Evaluate", func() {
		It("successfully evaluates a service instance", func() {
			req := policy.EvaluateRequest{
				Spec: map[string]any{
					"cpu":    "2",
					"memory": "4Gi",
				},
			}

			resp, err := client.Evaluate(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.Status).To(Equal("APPROVED"))
			Expect(resp.SelectedProvider).To(Equal("aws"))
			Expect(resp.EvaluatedSpec).To(HaveKey("cpu"))
			Expect(resp.EvaluatedSpec).To(HaveKey("memory"))
		})

		It("handles MODIFIED status", func() {
			response = enginev1alpha1.EvaluateResponse{
				Status:           enginev1alpha1.MODIFIED,
				SelectedProvider: "gcp",
				EvaluatedServiceInstance: enginev1alpha1.ServiceInstance{
					Spec: map[string]interface{}{
						"cpu":    "4",
						"memory": "8Gi",
					},
				},
			}

			req := policy.EvaluateRequest{
				Spec: map[string]any{
					"cpu":    "2",
					"memory": "4Gi",
				},
			}

			resp, err := client.Evaluate(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Status).To(Equal("MODIFIED"))
			Expect(resp.SelectedProvider).To(Equal("gcp"))
			Expect(resp.EvaluatedSpec["cpu"]).To(Equal("4"))
			Expect(resp.EvaluatedSpec["memory"]).To(Equal("8Gi"))
		})

		It("returns error for non-200 response", func() {
			statusCode = http.StatusBadRequest
			response = enginev1alpha1.BadRequest{
				Type:   "bad-request",
				Title:  "Bad Request",
				Status: 400,
			}

			req := policy.EvaluateRequest{
				Spec: map[string]any{"cpu": "invalid"},
			}

			_, err := client.Evaluate(ctx, req)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("policy engine returned status 400"))
		})

		It("returns HTTPError that can be extracted with errors.As", func() {
			statusCode = http.StatusNotAcceptable
			response = map[string]any{
				"type":   "policy-rejected",
				"title":  "Policy Rejected",
				"status": 406,
			}

			req := policy.EvaluateRequest{
				Spec: map[string]any{"cpu": "100"},
			}

			_, err := client.Evaluate(ctx, req)

			Expect(err).To(HaveOccurred())

			// Verify we can extract HTTPError using errors.As
			var httpErr *policy.HTTPError
			Expect(errors.As(err, &httpErr)).To(BeTrue(), "errors.As should be able to extract policy.HTTPError")
			Expect(httpErr).NotTo(BeNil())
			Expect(httpErr.StatusCode).To(Equal(406))
			Expect(httpErr.Body).To(ContainSubstring("policy-rejected"))
		})

		It("returns HTTPError for 500 internal server error", func() {
			statusCode = http.StatusInternalServerError
			response = map[string]any{
				"type":   "internal-error",
				"title":  "Internal Server Error",
				"status": 500,
			}

			req := policy.EvaluateRequest{
				Spec: map[string]any{"cpu": "2"},
			}

			_, err := client.Evaluate(ctx, req)

			Expect(err).To(HaveOccurred())

			// Verify HTTPError can be extracted
			var httpErr *policy.HTTPError
			Expect(errors.As(err, &httpErr)).To(BeTrue())
			Expect(httpErr.StatusCode).To(Equal(500))
		})
	})

	Describe("NewClient", func() {
		It("creates client with valid base URL", func() {
			client, err := policy.NewClient("http://localhost:8080")

			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})
	})
})
