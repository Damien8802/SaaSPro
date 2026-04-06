package models

import (
    "time"
)

// NotificationSettings представляет настройки уведомлений пользователя
type NotificationSettings struct {
    UserID          string    `json:"user_id"`
    TelegramEnabled bool      `json:"telegram_enabled"`
    EmailEnabled    bool      `json:"email_enabled"`
    Events          []string  `json:"events"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// NotificationLog запись в логе уведомлений
type NotificationLog struct {
    ID        string                 `json:"id"`
    UserID    string                 `json:"user_id"`
    Type      string                 `json:"type"`
    Details   map[string]interface{} `json:"details"`
    CreatedAt time.Time              `json:"created_at"`
}
