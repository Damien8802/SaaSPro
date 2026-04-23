package models

import "time"

type VPNKey struct {
    ID         int       `json:"id"`
    ClientName string    `json:"client_name"`
    ClientIP   string    `json:"client_ip"`
    PrivateKey string    `json:"private_key"`
    PublicKey  string    `json:"public_key"`
    PlanID     int       `json:"plan_id"`
    PlanName   string    `json:"plan_name"`
    ExpiresAt  time.Time `json:"expires_at"`
    Active     bool      `json:"active"`
    CreatedAt  time.Time `json:"created_at"`
    TenantID   string    `json:"tenant_id"`
}

type VPNPlan struct {
    ID       int     `json:"id"`
    Name     string  `json:"name"`
    Price    float64 `json:"price"`
    Days     int     `json:"days"`
    Speed    string  `json:"speed"`
    Devices  int     `json:"devices"`
    TenantID string  `json:"tenant_id"`
}

type VPNStats struct {
    TotalKeys   int `json:"total_keys"`
    ActiveKeys  int `json:"active_keys"`
    ExpiredKeys int `json:"expired_keys"`
    NearExpiry  int `json:"near_expiry"`
}