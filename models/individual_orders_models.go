package models

import (
    "time"
    "github.com/google/uuid"
)

type ServiceCategory struct {
    ID          uuid.UUID `gorm:"primaryKey" json:"id"`
    Name        string    `json:"name"`
    Slug        string    `json:"slug"`
    Description string    `json:"description"`
    Icon        string    `json:"icon"`
    SortOrder   int       `json:"sort_order"`
    IsActive    bool      `json:"is_active"`
    CreatedAt   time.Time `json:"created_at"`
}

func (ServiceCategory) TableName() string {
    return "service_categories"
}

type Service struct {
    ID            uuid.UUID  `gorm:"primaryKey" json:"id"`
    CategoryID    *uuid.UUID `json:"category_id,omitempty"`
    Name          string     `json:"name"`
    Slug          string     `json:"slug"`
    Description   string     `json:"description"`
    BasePrice     *float64   `json:"base_price"`
    PriceRangeMin *float64   `json:"price_range_min"`
    PriceRangeMax *float64   `json:"price_range_max"`
    DeliveryDays  int        `json:"delivery_days"`
    IsActive      bool       `json:"is_active"`
    SortOrder     int        `json:"sort_order"`
    CreatedAt     time.Time  `json:"created_at"`
}

func (Service) TableName() string {
    return "services"
}

type IndividualOrder struct {
    ID             uuid.UUID  `gorm:"primaryKey" json:"id"`
    CompanyID      *uuid.UUID `json:"company_id,omitempty"`
    UserID         *uuid.UUID `json:"user_id,omitempty"`
    ServiceID      *uuid.UUID `json:"service_id,omitempty"`
    ServiceName    string     `json:"service_name"`
    Requirements   string     `json:"requirements"`
    EstimatedPrice *float64   `json:"estimated_price"`
    FinalPrice     *float64   `json:"final_price"`
    Status         string     `json:"status"`
    ClientName     string     `json:"client_name"`
    ClientPhone    string     `json:"client_phone"`
    ClientEmail    string     `json:"client_email"`
    ClientTelegram string     `json:"client_telegram"`
    AdminComment   string     `json:"admin_comment"`
    CreatedAt      time.Time  `json:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at"`
}

func (IndividualOrder) TableName() string {
    return "individual_orders"
}

const (
    OrderStatusPending    = "pending"
    OrderStatusApproved   = "approved"
    OrderStatusInProgress = "in_progress"
    OrderStatusCompleted  = "completed"
    OrderStatusCancelled  = "cancelled"
)
