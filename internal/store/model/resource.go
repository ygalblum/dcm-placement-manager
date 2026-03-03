package model

import (
	"time"

	"github.com/google/uuid"
)

// Resource represents a resource provisioning request
type Resource struct {
	ID                    uuid.UUID       `gorm:"column:id;primaryKey;type:uuid"`
	CatalogItemInstanceId string          `gorm:"column:catalog_item_instance_id;not null"`
	OriginalSpec          map[string]any  `gorm:"column:original_spec;type:jsonb;serializer:json;not null"`
	ProviderName          *string         `gorm:"column:provider_name"`
	ApprovalStatus        *string         `gorm:"column:approval_status"`
	ValidSpec             *map[string]any `gorm:"column:valid_spec;type:jsonb;serializer:json"`
	Path                  string          `gorm:"column:path;not null"`
	CreateTime            time.Time       `gorm:"column:create_time;autoCreateTime"`
	UpdateTime            time.Time       `gorm:"column:update_time;autoUpdateTime"`
}

// ResourceList is a list of requests
type ResourceList []Resource
