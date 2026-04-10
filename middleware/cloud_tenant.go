package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// CloudTenantMiddleware - защита данных в облачном хранилище
// Клиент видит только свои файлы, не может получить доступ к чужим
func CloudTenantMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
        }
        
        // Для GET запросов с file_id - проверяем принадлежность файла
        fileID := c.Param("id")
        if fileID != "" && c.Request.Method == "GET" {
            var ownerID string
            err := database.Pool.QueryRow(c.Request.Context(), `
                SELECT user_id FROM cloud_files WHERE id = $1
            `, fileID).Scan(&ownerID)
            
            if err == nil && ownerID != userID {
                c.JSON(http.StatusForbidden, gin.H{
                    "error": "Access denied: you don't own this file",
                })
                c.Abort()
                return
            }
        }
        
        // Для DELETE запросов - проверяем владельца
        if fileID != "" && c.Request.Method == "DELETE" {
            var ownerID string
            err := database.Pool.QueryRow(c.Request.Context(), `
                SELECT user_id FROM cloud_files WHERE id = $1
            `, fileID).Scan(&ownerID)
            
            if err == nil && ownerID != userID {
                c.JSON(http.StatusForbidden, gin.H{
                    "error": "Access denied: you cannot delete someone else's file",
                })
                c.Abort()
                return
            }
        }
        
        c.Set("cloud_user_id", userID)
        c.Next()
    }
}

// CloudBucketMiddleware - защита бакетов
func CloudBucketMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
        }
        
        bucketID := c.Param("bucket_id")
        if bucketID != "" {
            var ownerID string
            err := database.Pool.QueryRow(c.Request.Context(), `
                SELECT user_id FROM cloud_buckets WHERE id = $1
            `, bucketID).Scan(&ownerID)
            
            if err == nil && ownerID != userID {
                c.JSON(http.StatusForbidden, gin.H{
                    "error": "Access denied: this bucket belongs to another user",
                })
                c.Abort()
                return
            }
        }
        
        c.Next()
    }
}