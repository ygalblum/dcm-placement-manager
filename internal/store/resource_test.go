package store_test

import (
	"context"

	"github.com/dcm-project/placement-manager/internal/store"
	"github.com/dcm-project/placement-manager/internal/store/model"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("Resource Store", func() {
	var (
		db           *gorm.DB
		requestStore store.Resource
		ctx          context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Resource{})).To(Succeed())

		requestStore = store.NewResource(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	Describe("Create", func() {
		It("persists the resource without optional fields", func() {
			r := model.Resource{
				ID:                    uuid.New(),
				CatalogItemInstanceId: "catalog-instance-123",
				OriginalSpec:          map[string]any{"cpu": "2", "memory": "4Gi"},
			}
			created, err := requestStore.Create(ctx, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.ID).To(Equal(r.ID))
			Expect(created.CatalogItemInstanceId).To(Equal("catalog-instance-123"))
			Expect(created.OriginalSpec).To(Equal(map[string]any{"cpu": "2", "memory": "4Gi"}))
			Expect(created.ProviderName).To(BeNil())
			Expect(created.ApprovalStatus).To(BeNil())
			Expect(created.ValidSpec).To(BeNil())
		})
	})

	Describe("Get", func() {
		It("retrieves by ID", func() {
			r := model.Resource{
				ID:                    uuid.New(),
				CatalogItemInstanceId: "catalog-instance-456",
				OriginalSpec:          map[string]any{"test": "data"},
			}
			_, _ = requestStore.Create(ctx, r)

			found, err := requestStore.Get(ctx, r.ID)

			Expect(err).NotTo(HaveOccurred())
			Expect(found.CatalogItemInstanceId).To(Equal("catalog-instance-456"))
		})

		It("returns ErrRequestNotFound for missing ID", func() {
			_, err := requestStore.Get(ctx, uuid.New())

			Expect(err).To(Equal(store.ErrRequestNotFound))
		})
	})

	Describe("Update", func() {
		var testRequest *model.Resource

		BeforeEach(func() {
			// Create a test resource for all Update tests
			r := model.Resource{
				ID:                    uuid.New(),
				CatalogItemInstanceId: "catalog-update-test",
				OriginalSpec:          map[string]any{"cpu": "2"},
			}
			created, err := requestStore.Create(ctx, r)
			Expect(err).NotTo(HaveOccurred())
			testRequest = created
		})

		It("updates provider name, approval status, and spec", func() {
			// Update with provider name, approval status, and spec
			providerName := "updated-provider"
			approvalStatus := "modified"
			spec := map[string]any{"cpu": "4", "memory": "8Gi"}
			testRequest.ProviderName = &providerName
			testRequest.ApprovalStatus = &approvalStatus
			testRequest.ValidSpec = &spec

			updated, err := requestStore.Update(ctx, *testRequest)

			Expect(err).NotTo(HaveOccurred())
			Expect(*updated.ProviderName).To(Equal("updated-provider"))
			Expect(*updated.ApprovalStatus).To(Equal("modified"))
			Expect(*updated.ValidSpec).To(Equal(map[string]any{"cpu": "4", "memory": "8Gi"}))
		})

		It("validates approval status on update", func() {
			// Try to update with invalid approval status
			invalidStatus := "pending"
			testRequest.ApprovalStatus = &invalidStatus

			_, err := requestStore.Update(ctx, *testRequest)

			Expect(err).To(Equal(store.ErrInvalidApprovalStatus))
		})

		It("accepts 'approved' approval status", func() {
			// Update with approved status
			approvedStatus := "approved"
			testRequest.ApprovalStatus = &approvedStatus

			updated, err := requestStore.Update(ctx, *testRequest)

			Expect(err).NotTo(HaveOccurred())
			Expect(*updated.ApprovalStatus).To(Equal("approved"))
		})

		It("returns ErrRequestNotFound for missing ID", func() {
			r := model.Resource{
				ID:                    uuid.New(),
				CatalogItemInstanceId: "non-existent",
				OriginalSpec:          map[string]any{},
			}

			_, err := requestStore.Update(ctx, r)

			Expect(err).To(Equal(store.ErrRequestNotFound))
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			provider1 := "provider-a"
			provider2 := "provider-b"
			// Create test data
			requests := []model.Resource{
				{ID: uuid.New(), ProviderName: &provider1, CatalogItemInstanceId: "cat-1", OriginalSpec: map[string]any{}},
				{ID: uuid.New(), ProviderName: &provider2, CatalogItemInstanceId: "cat-2", OriginalSpec: map[string]any{}},
				{ID: uuid.New(), ProviderName: &provider1, CatalogItemInstanceId: "cat-3", OriginalSpec: map[string]any{}},
			}
			for _, r := range requests {
				_, err := requestStore.Create(ctx, r)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("returns all requests when opts is nil", func() {
			result, err := requestStore.List(ctx, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resources).To(HaveLen(3))
			Expect(result.NextPageToken).To(BeNil())
		})

		It("filters by provider name", func() {
			providerName := "provider-a"
			opts := &store.ResourceListOptions{
				ProviderName: &providerName,
			}

			result, err := requestStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			for _, r := range result.Resources {
				Expect(*r.ProviderName).To(Equal("provider-a"))
			}
		})

		It("supports pagination with page size", func() {
			opts := &store.ResourceListOptions{
				PageSize: 2,
			}

			result, err := requestStore.List(ctx, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			Expect(result.NextPageToken).NotTo(BeNil())
		})

		It("supports pagination with page token", func() {
			// Get first page
			opts1 := &store.ResourceListOptions{
				PageSize: 2,
			}
			result1, err := requestStore.List(ctx, opts1)
			Expect(err).NotTo(HaveOccurred())
			Expect(result1.NextPageToken).NotTo(BeNil())

			// Get second page
			opts2 := &store.ResourceListOptions{
				PageSize:  2,
				PageToken: result1.NextPageToken,
			}
			result2, err := requestStore.List(ctx, opts2)

			Expect(err).NotTo(HaveOccurred())
			Expect(result2.Resources).To(HaveLen(1))
			Expect(result2.NextPageToken).To(BeNil())
		})
	})

	Describe("Delete", func() {
		It("deletes the resource", func() {
			r := model.Resource{
				ID:                    uuid.New(),
				CatalogItemInstanceId: "cat-del",
				OriginalSpec:          map[string]any{},
			}
			_, _ = requestStore.Create(ctx, r)

			err := requestStore.Delete(ctx, r.ID)

			Expect(err).NotTo(HaveOccurred())

			_, err = requestStore.Get(ctx, r.ID)
			Expect(err).To(Equal(store.ErrRequestNotFound))
		})

		It("returns ErrRequestNotFound for missing ID", func() {
			err := requestStore.Delete(ctx, uuid.New())

			Expect(err).To(Equal(store.ErrRequestNotFound))
		})
	})
})
