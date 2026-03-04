package handlers_test

import (
	"context"

	"github.com/dcm-project/placement-manager/internal/api/server"
	"github.com/dcm-project/placement-manager/internal/handlers"
	"github.com/dcm-project/placement-manager/internal/service"
	"github.com/dcm-project/placement-manager/internal/store"
	"github.com/dcm-project/placement-manager/internal/store/model"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("Handler", func() {
	var (
		db      *gorm.DB
		handler *handlers.Handler
		ctx     context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Resource{})).To(Succeed())

		dataStore := store.NewStore(db)
		placementService := service.NewPlacementService(dataStore)
		handler = handlers.NewHandler(placementService)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	Describe("GetHealth", func() {
		It("returns ok status", func() {
			resp, err := handler.GetHealth(ctx, server.GetHealthRequestObject{})

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.GetHealth200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Status).To(Equal("ok"))
			Expect(*jsonResp.Path).To(Equal("health"))
		})
	})

	Describe("CreateResource", func() {
		It("creates and returns 201", func() {
			req := server.CreateResourceRequestObject{
				Body: &server.Resource{
					CatalogItemInstanceId: "catalog-item-123",
					Spec:                  map[string]interface{}{"cpu": 2, "memory": "4GB"},
				},
			}

			resp, err := handler.CreateResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.CreateResource201JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.CatalogItemInstanceId).To(Equal("catalog-item-123"))
			Expect(jsonResp.Id).NotTo(BeNil())
			Expect(jsonResp.Path).NotTo(BeNil())
			Expect(jsonResp.Spec).To(HaveLen(2))
			Expect(jsonResp.Spec).To(HaveKey("cpu"))
			Expect(jsonResp.Spec).To(HaveKey("memory"))
			Expect(jsonResp.Spec["memory"]).To(Equal("4GB"))
		})

		It("creates with specified ID", func() {
			specifiedID := uuid.New().String()
			req := server.CreateResourceRequestObject{
				Params: server.CreateResourceParams{Id: &specifiedID},
				Body: &server.Resource{
					CatalogItemInstanceId: "catalog-item-456",
					Spec:                  map[string]interface{}{"cpu": 1},
				},
			}

			resp, err := handler.CreateResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.CreateResource201JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Id).To(Equal(specifiedID))
		})
	})

	Describe("GetResource", func() {
		It("returns resource", func() {
			// Create a resource first
			createReq := server.CreateResourceRequestObject{
				Body: &server.Resource{
					CatalogItemInstanceId: "catalog-instance-1234",
					Spec:                  map[string]interface{}{"cpu": 2},
				},
			}
			createResp, _ := handler.CreateResource(ctx, createReq)
			created := createResp.(server.CreateResource201JSONResponse)

			req := server.GetResourceRequestObject{
				ResourceId: *created.Id,
			}

			resp, err := handler.GetResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.GetResource200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.CatalogItemInstanceId).To(Equal("catalog-instance-1234"))
		})

		It("returns 404 for non-existent resource", func() {
			req := server.GetResourceRequestObject{
				ResourceId: uuid.New().String(),
			}

			resp, err := handler.GetResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.GetResource404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 400 for invalid ID format", func() {
			req := server.GetResourceRequestObject{
				ResourceId: "not-a-uuid",
			}

			resp, err := handler.GetResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.GetResource400ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("ListResources", func() {
		It("returns empty list initially", func() {
			req := server.ListResourcesRequestObject{}

			resp, err := handler.ListResources(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListResources200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Resources).To(BeEmpty())
		})

		It("returns resources", func() {
			// Create resources first
			for i := 0; i < 3; i++ {
				createReq := server.CreateResourceRequestObject{
					Body: &server.Resource{
						CatalogItemInstanceId: "catalog-instance-123",
						Spec:                  map[string]interface{}{"cpu": i + 1},
					},
				}
				_, err := handler.CreateResource(ctx, createReq)
				Expect(err).NotTo(HaveOccurred())
			}

			resp, err := handler.ListResources(ctx, server.ListResourcesRequestObject{})

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListResources200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Resources).To(HaveLen(3))
		})

		It("respects max page size and returns next page token", func() {
			// Create 5 resources
			for i := 0; i < 5; i++ {
				createReq := server.CreateResourceRequestObject{
					Body: &server.Resource{
						CatalogItemInstanceId: "catalog-instance-123",
						Spec:                  map[string]interface{}{"cpu": i + 1},
					},
				}
				_, err := handler.CreateResource(ctx, createReq)
				Expect(err).NotTo(HaveOccurred())
			}

			// First page: request 2 items
			maxPageSize := 2
			req := server.ListResourcesRequestObject{
				Params: server.ListResourcesParams{MaxPageSize: &maxPageSize},
			}

			resp, err := handler.ListResources(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListResources200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Resources).To(HaveLen(2))
			Expect(jsonResp.NextPageToken).NotTo(BeNil())
			Expect(*jsonResp.NextPageToken).NotTo(BeEmpty())

			// Second page: use the page token
			req2 := server.ListResourcesRequestObject{
				Params: server.ListResourcesParams{
					MaxPageSize: &maxPageSize,
					PageToken:   jsonResp.NextPageToken,
				},
			}

			resp2, err := handler.ListResources(ctx, req2)

			Expect(err).NotTo(HaveOccurred())
			jsonResp2, ok := resp2.(server.ListResources200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp2.Resources).To(HaveLen(2))
			Expect(jsonResp2.NextPageToken).NotTo(BeNil())

			// Third page: should have 1 item and no next token
			req3 := server.ListResourcesRequestObject{
				Params: server.ListResourcesParams{
					MaxPageSize: &maxPageSize,
					PageToken:   jsonResp2.NextPageToken,
				},
			}

			resp3, err := handler.ListResources(ctx, req3)

			Expect(err).NotTo(HaveOccurred())
			jsonResp3, ok := resp3.(server.ListResources200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp3.Resources).To(HaveLen(1))
			Expect(jsonResp3.NextPageToken).To(BeNil())
		})

		It("returns 400 for invalid page size", func() {
			invalidPageSize := 200
			req := server.ListResourcesRequestObject{
				Params: server.ListResourcesParams{MaxPageSize: &invalidPageSize},
			}

			resp, err := handler.ListResources(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.ListResources400ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("DeleteResource", func() {
		It("deletes resource and returns 204", func() {
			// Create a resource first
			createReq := server.CreateResourceRequestObject{
				Body: &server.Resource{
					CatalogItemInstanceId: "catalog-delete",
					Spec:                  map[string]interface{}{"cpu": 2},
				},
			}
			createResp, _ := handler.CreateResource(ctx, createReq)
			created := createResp.(server.CreateResource201JSONResponse)

			req := server.DeleteResourceRequestObject{
				ResourceId: *created.Id,
			}

			resp, err := handler.DeleteResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteResource204Response)
			Expect(ok).To(BeTrue())

			// Verify it's deleted
			getResp, _ := handler.GetResource(ctx, server.GetResourceRequestObject{ResourceId: *created.Id})
			_, ok = getResp.(server.GetResource404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 404 for non-existent resource", func() {
			req := server.DeleteResourceRequestObject{
				ResourceId: uuid.New().String(),
			}

			resp, err := handler.DeleteResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteResource404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 400 for invalid ID format", func() {
			req := server.DeleteResourceRequestObject{
				ResourceId: "invalid-uuid",
			}

			resp, err := handler.DeleteResource(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteResource400ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})
})
