package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// Require2FA - требует прохождения 2FA для админских маршрутов
func Require2FA() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        role := c.GetString("role")
        
        // Только для админов и разработчиков
        if role != "admin" && role != "developer" {
            c.Next()
            return
        }
        
        // Проверяем, включена ли 2FA
        var enabled bool
        err := database.Pool.QueryRow(c.Request.Context(),
            "SELECT enabled FROM twofa WHERE user_id = $1", userID).Scan(&enabled)
        
        if err == nil && enabled {
            // Проверяем, пройдена ли 2FA в этой сессии
            twofaPassed := c.GetBool("2fa_passed")
            if !twofaPassed {
                c.JSON(http.StatusForbidden, gin.H{
                    "error": "Требуется двухфакторная аутентификация",
                    "require_2fa": true,
                })
                c.Abort()
                return
            }
        }
        
        c.Next()
    }
}