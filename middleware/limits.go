package middleware

import (
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// CheckDealsLimit проверяет лимит сделок
func CheckDealsLimit() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, exists := c.Get("userID")
        if !exists {
            c.Next()
            return
        }
        
        var dealsLimit int
        var currentDeals int
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT p.deals_limit, COUNT(d.id)
            FROM users u
            JOIN plans p ON p.id = u.plan_id
            LEFT JOIN crm_deals d ON d.user_id = u.id
            WHERE u.id = $1
            GROUP BY p.deals_limit
        `, userID).Scan(&dealsLimit, &currentDeals)
        
        if err == nil && currentDeals >= dealsLimit {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "Превышен лимит сделок. Обновите тариф.",
                "limit": dealsLimit,
                "current": currentDeals,
            })
            c.Abort()
            return
        }
        c.Next()
    }
}