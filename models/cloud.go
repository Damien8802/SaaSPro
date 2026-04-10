package models

import (
    "time"
    "github.com/google/uuid"
)

type CloudFile struct {
    ID         uuid.UUID `json:"id" db:"id"`
    TenantID   uuid.UUID `json:"tenant_id" db:"tenant_id"`
    UserID     uuid.UUID `json:"user_id" db:"user_id"`
    Name       string    `json:"name" db:"name"`
    Path       string    `json:"path" db:"path"`
    Size       int64     `json:"size" db:"size"`
    MimeType   string    `json:"mime_type" db:"mime_type"`
    Folder     string    `json:"folder" db:"folder"`
    IsStarred  bool      `json:"is_starred" db:"is_starred"`
    IsShared   bool      `json:"is_shared" db:"is_shared"`
    IsActive   bool      `json:"is_active" db:"is_active"`
    CreatedAt  time.Time `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type CloudFolder struct {
    ID        uuid.UUID  `json:"id" db:"id"`
    TenantID  uuid.UUID  `json:"tenant_id" db:"tenant_id"`
    UserID    uuid.UUID  `json:"user_id" db:"user_id"`
    Name      string     `json:"name" db:"name"`
    ParentID  *uuid.UUID `json:"parent_id" db:"parent_id"`
    Path      string     `json:"path" db:"path"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type CloudPlan struct {
    ID           uuid.UUID `json:"id" db:"id"`
    Name         string    `json:"name" db:"name"`
    QuotaGB      int       `json:"quota_gb" db:"quota_gb"`
    PriceMonthly float64   `json:"price_monthly" db:"price_monthly"`
    PriceYearly  float64   `json:"price_yearly" db:"price_yearly"`
    IsFreeForDev bool      `json:"is_free_for_dev" db:"is_free_for_dev"`
    SortOrder    int       `json:"sort_order" db:"sort_order"`
    IsActive     bool      `json:"is_active" db:"is_active"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

func (CloudFile) TableName() string   { return "cloud_files" }
func (CloudFolder) TableName() string { return "cloud_folders" }
func (CloudPlan) TableName() string   { return "cloud_plans" }