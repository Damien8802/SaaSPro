package models

import (
    "database/sql"
    "time"
)

type Event struct {
    ID          string         `json:"id" db:"id"`
    Title       string         `json:"title" db:"title"`
    Description string         `json:"description" db:"description"`
    StartTime   time.Time      `json:"start_time" db:"start_time"`
    EndTime     sql.NullTime   `json:"end_time,omitempty" db:"end_time"`
    UserID      string         `json:"user_id" db:"user_id"`
    DealID      sql.NullString `json:"deal_id,omitempty" db:"deal_id"`
    CustomerID  sql.NullString `json:"customer_id,omitempty" db:"customer_id"`
    Type        string         `json:"type" db:"type"`
    Status      string         `json:"status" db:"status"`
    CreatedAt   time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}