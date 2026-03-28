package models

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

// Tenant - компания (тенант)
type Tenant struct {
    ID        uuid.UUID       `json:"id"`
    Name      string          `json:"name"`
    Subdomain string          `json:"subdomain"`
    LogoURL   string          `json:"logo_url"`
    Settings  json.RawMessage `json:"settings"`
    Status    string          `json:"status"`
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
}

// TenantSettings - настройки тенанта
type TenantSettings struct {
    Theme        string `json:"theme"`
    Language     string `json:"language"`
    Timezone     string `json:"timezone"`
    MaxUsers     int    `json:"max_users"`
    MaxProjects  int    `json:"max_projects"`
    MaxTasks     int    `json:"max_tasks"`
    MaxStorageMB int    `json:"max_storage_mb"`
}

// GetSettings - получить настройки тенанта
func (t *Tenant) GetSettings() (*TenantSettings, error) {
    var settings TenantSettings
    if len(t.Settings) == 0 {
        // Настройки по умолчанию
        return &TenantSettings{
            Theme:        "dark",
            Language:     "ru",
            Timezone:     "Europe/Moscow",
            MaxUsers:     50,
            MaxProjects:  100,
            MaxTasks:     1000,
            MaxStorageMB: 1024,
        }, nil
    }
    err := json.Unmarshal(t.Settings, &settings)
    return &settings, err
}