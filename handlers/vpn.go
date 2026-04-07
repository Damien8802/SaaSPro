package handlers

import (
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// CreateVPNTables создаёт таблицы для VPN
func CreateVPNTables(c *gin.Context) {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS vpn_keys (
            id SERIAL PRIMARY KEY,
            client_name VARCHAR(100) UNIQUE NOT NULL,
            client_ip VARCHAR(15) NOT NULL,
            private_key TEXT NOT NULL,
            public_key TEXT NOT NULL,
            plan_id INTEGER REFERENCES vpn_plans(id),
            expires_at TIMESTAMP NOT NULL,
            active BOOLEAN DEFAULT TRUE,
            created_at TIMESTAMP DEFAULT NOW()
        );`,
        
        `CREATE INDEX IF NOT EXISTS idx_vpn_keys_client_name ON vpn_keys(client_name);`,
        `CREATE INDEX IF NOT EXISTS idx_vpn_keys_expires_at ON vpn_keys(expires_at);`,
        `CREATE INDEX IF NOT EXISTS idx_vpn_keys_active ON vpn_keys(active);`,
    }
    
    for _, q := range queries {
        if _, err := database.Pool.Exec(c.Request.Context(), q); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "query": q})
            return
        }
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "VPN таблицы созданы"})
}