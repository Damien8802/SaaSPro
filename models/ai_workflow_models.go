package models

import (
    "encoding/json"
    "time"
)

// AIWorkflow - модель для хранения workflow (НОВАЯ)
type AIWorkflow struct {
    ID           string          `gorm:"primaryKey" json:"id"`
    TenantID     string          `gorm:"index" json:"tenant_id"`
    Name         string          `json:"name"`
    TriggerEvent string          `json:"trigger_event"`
    Actions      json.RawMessage `json:"actions"`
    IsActive     bool            `json:"is_active"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
}

func (AIWorkflow) TableName() string {
    return "ai_workflows"
}

// AIActionHistory - история действий AI (НОВАЯ)
type AIActionHistory struct {
    ID           string          `gorm:"primaryKey" json:"id"`
    TenantID     string          `gorm:"index" json:"tenant_id"`
    UserID       string          `gorm:"index" json:"user_id"`
    ActionType   string          `json:"action_type"`
    ActionData   json.RawMessage `json:"action_data"`
    Result       json.RawMessage `json:"result"`
    Status       string          `json:"status"`
    ErrorMessage string          `json:"error_message"`
    CreatedAt    time.Time       `json:"created_at"`
}

func (AIActionHistory) TableName() string {
    return "ai_action_history"
}