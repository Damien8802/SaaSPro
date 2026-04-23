package middleware

import (
    "context"
    "net/http"
    
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// RequireModuleAccess - проверяет доступ к модулю
func RequireModuleAccess(moduleCode string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("role")
        
        // Разработчик имеет полный доступ
        if role == "developer" {
            c.Next()
            return
        }
        
        tenantID := c.GetString("tenant_id")
        if tenantID == "" {
            c.JSON(http.StatusForbidden, gin.H{
                "error":   "Доступ запрещён",
                "message": "Tenant ID не найден",
            })
            c.Abort()
            return
        }
        
        // Проверяем подписку
        var isActive bool
        err := database.Pool.QueryRow(context.Background(), `
            SELECT EXISTS (
                SELECT 1 FROM module_subscriptions 
                WHERE tenant_id = $1 
                AND module_code = $2 
                AND status = 'active'
            )
        `, tenantID, moduleCode).Scan(&isActive)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Ошибка проверки доступа",
            })
            c.Abort()
            return
        }
        
        if !isActive {
            c.JSON(http.StatusPaymentRequired, gin.H{
                "error":       "Модуль не оплачен",
                "message":     "Для доступа к этому модулю необходимо оплатить подписку",
                "module":      moduleCode,
                "upgrade_url": "/pricing",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// HasModuleAccess - проверяет доступ к модулю (для использования в коде)
func HasModuleAccess(tenantID, moduleCode string) bool {
    var isActive bool
    err := database.Pool.QueryRow(context.Background(), `
        SELECT EXISTS (
            SELECT 1 FROM module_subscriptions 
            WHERE tenant_id = $1 
            AND module_code = $2 
            AND status = 'active'
        )
    `, tenantID, moduleCode).Scan(&isActive)
    
    return err == nil && isActive
}

// GetAvailableModules - возвращает список доступных модулей для tenant
func GetAvailableModules(tenantID string) []string {
    if tenantID == "" {
        return []string{}
    }
    
    rows, err := database.Pool.Query(context.Background(), `
        SELECT module_code FROM module_subscriptions 
        WHERE tenant_id = $1 AND status = 'active'
    `, tenantID)
    if err != nil {
        return []string{}
    }
    defer rows.Close()
    
    var modules []string
    for rows.Next() {
        var module string
        rows.Scan(&module)
        modules = append(modules, module)
    }
    return modules
}