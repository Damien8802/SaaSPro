package models

import (
	"time"
)

// AIAgent - структура ИИ-агента
type AIAgent struct {
	ID           string    `json:"id" db:"id"`
	AccountID    string    `json:"account_id" db:"account_id"`
	Name         string    `json:"name" db:"name"`
	Role         string    `json:"role" db:"role"`
	Instructions string    `json:"instructions" db:"instructions"`
	Model        string    `json:"model" db:"model"`
	Temperature  float64   `json:"temperature" db:"temperature"`
	Schedule     string    `json:"schedule" db:"schedule"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// AIAgentAction - действия агента
type AIAgentAction struct {
	ID        string                 `json:"id" db:"id"`
	AgentID   string                 `json:"agent_id" db:"agent_id"`
	Action    string                 `json:"action" db:"action"`
	Condition string                 `json:"condition" db:"condition"`
	Config    map[string]interface{} `json:"config" db:"config"`
	IsActive  bool                   `json:"is_active" db:"is_active"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

// AIAgentLog - лог действий
type AIAgentLog struct {
	ID         string    `json:"id" db:"id"`
	AgentID    string    `json:"agent_id" db:"agent_id"`
	Action     string    `json:"action" db:"action"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	DealID     string    `json:"deal_id" db:"deal_id"`
	Result     string    `json:"result" db:"result"`
	Status     string    `json:"status" db:"status"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}