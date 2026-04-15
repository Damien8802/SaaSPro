package models

import (
    "time"
    "github.com/google/uuid"
)

type AIConversation struct {
    ID             uuid.UUID  `gorm:"primaryKey" json:"id"`
    CompanyID      *uuid.UUID `json:"company_id,omitempty"`
    UserID         *uuid.UUID `json:"user_id,omitempty"`
    SessionID      string     `json:"session_id"`
    Message        string     `json:"message"`
    Response       string     `json:"response"`
    Intent         string     `json:"intent"`
    ActionTaken    string     `json:"action_taken"`
    ModuleUsed     string     `json:"module_used"`
    TokensUsed     int        `json:"tokens_used"`
    ResponseTimeMs int        `json:"response_time_ms"`
    CreatedAt      time.Time  `json:"created_at"`
}

func (AIConversation) TableName() string {
    return "ai_conversations"
}

type AIAction struct {
    ID              uuid.UUID `gorm:"primaryKey" json:"id"`
    ModuleCode      string    `json:"module_code"`
    ActionCode      string    `json:"action_code"`
    Name            string    `json:"name"`
    Description     string    `json:"description"`
    Example         string    `json:"example"`
    RequiresPayment bool      `json:"requires_payment"`
    Endpoint        string    `json:"endpoint"`
    IsActive        bool      `json:"is_active"`
    SortOrder       int       `json:"sort_order"`
    CreatedAt       time.Time `json:"created_at"`
}

func (AIAction) TableName() string {
    return "ai_actions"
}

type PriceCache struct {
    ID            uuid.UUID   `gorm:"primaryKey" json:"id"`
    QueryHash     string      `json:"query_hash"`
    Query         string      `json:"query"`
    AvgPrice      *float64    `json:"avg_price"`
    MinPrice      *float64    `json:"min_price"`
    MaxPrice      *float64    `json:"max_price"`
    SourcesCount  int         `json:"sources_count"`
    SearchResults interface{} `gorm:"type:jsonb" json:"search_results"`
    CreatedAt     time.Time   `json:"created_at"`
    ExpiresAt     time.Time   `json:"expires_at"`
}

func (PriceCache) TableName() string {
    return "price_cache"
}
