package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

// TenantMiddleware - определяет тенанта по subdomain
func TenantMiddleware(db *pgxpool.Pool) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Определяем тенанта по subdomain из URL или заголовка
        host := c.Request.Host
        subdomain := extractSubdomain(host)
        
        // Проверяем, есть ли tenant в query параметре (для разработки)
        if tenantParam := c.Query("tenant"); tenantParam != "" {
            subdomain = tenantParam
        }
        
        var tenantID uuid.UUID
        var tenantName string
        var tenantSettings []byte
        
        // Ищем тенанта по subdomain
        err := db.QueryRow(c.Request.Context(), `
            SELECT id, name, settings FROM tenants 
            WHERE subdomain = $1 AND status = 'active'
        `, subdomain).Scan(&tenantID, &tenantName, &tenantSettings)
        
        if err != nil {
            // Если не нашли, используем дефолтного
            err = db.QueryRow(c.Request.Context(), `
                SELECT id, name, settings FROM tenants 
                WHERE subdomain = 'default'
            `).Scan(&tenantID, &tenantName, &tenantSettings)
            
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                    "error": "Tenant not found",
                })
                c.Abort()
                return
            }
        }
        
        // Сохраняем в контекст
        c.Set("tenant_id", tenantID)
        c.Set("tenant_name", tenantName)
        c.Set("tenant_subdomain", subdomain)
        
        // Добавляем tenant_id в заголовки для API
        c.Header("X-Tenant-ID", tenantID.String())
        c.Header("X-Tenant-Name", tenantName)
        
        c.Next()
    }
}

// extractSubdomain - извлекает subdomain из host
func extractSubdomain(host string) string {
    // Убираем порт
    if idx := strings.Index(host, ":"); idx != -1 {
        host = host[:idx]
    }
    
    parts := strings.Split(host, ".")
    if len(parts) >= 2 {
        // Если это localhost или IP, возвращаем default
        if host == "localhost" || strings.Contains(host, "127.0.0.1") {
            return "default"
        }
        return parts[0]
    }
    return "default"
}

// GetTenantIDFromContext - получить tenant_id из контекста
func GetTenantIDFromContext(c *gin.Context) uuid.UUID {
    if tenantID, exists := c.Get("tenant_id"); exists {
        if id, ok := tenantID.(uuid.UUID); ok {
            return id
        }
    }
    return uuid.Nil
}