package middleware

import (
    "log"
    "net/http"
    "strings"
    "subscription-system/config"
    "subscription-system/utils"

    "github.com/gin-gonic/gin"
)

// AuthMiddleware проверяет JWT, учитывая публичные маршруты и режим разработки
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        // ========== ПУБЛИЧНЫЕ МАРШРУТЫ ==========
        // Эти маршруты доступны без авторизации всегда
        publicRoutes := map[string]bool{
            // Публичные страницы
            "/":                        true,
            "/about":                   true,
            "/contact":                 true,
            "/info":                    true,
            "/pricing":                 true,
            "/partner":                 true,
            "/login":                   true,
            "/register":                true,
            "/forgot-password":         true,
            
            // Публичные API
            "/api/health":              true,
            "/api/crm/health":          true,
            "/api/test":                true,
            "/api/auth/login":          true,
            "/api/auth/register":       true,
            "/api/auth/refresh":        true,
            "/api/auth/logout":         true,
            
            // AI ассистент (должен работать без авторизации)
            "/api/crm/ai/ask":          true,
            "/api/ai/ask":              true,
        }

        // Проверяем публичные маршруты
        if publicRoutes[path] {
            if cfg.SkipAuth {
                log.Printf("[AUTH] Публичный маршрут %s %s (режим разработки)", method, path)
            }
            c.Next()
            return
        }

        // ========== РЕЖИМ РАЗРАБОТКИ ==========
        if cfg.SkipAuth {
            // В режиме разработки используем тестового администратора
            userID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
            c.Set("userID", userID)
            c.Set("role", "admin")
            
            log.Printf("[AUTH] 🟢 Режим разработки: %s %s, user=%s", method, path, userID)
            c.Next()
            return
        }

        // ========== ПРОДАКШЕН РЕЖИМ ==========
        // Проверка заголовка для пропуска авторизации (только для тестирования)
        if c.GetHeader("X-Skip-Auth") == "true" {
            log.Printf("[AUTH] ⚠️ Внимание: X-Skip-Auth использован для %s %s", method, path)
            c.Next()
            return
        }

        // Стандартная JWT авторизация
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            log.Printf("[AUTH] 🔴 Ошибка: нет Authorization header для %s %s", method, path)
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "authorization header required",
                "code":  "UNAUTHORIZED",
            })
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if !(len(parts) == 2 && strings.ToLower(parts[0]) == "bearer") {
            log.Printf("[AUTH] 🔴 Ошибка: неверный формат Authorization header для %s %s", method, path)
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid authorization header format. Use 'Bearer <token>'",
                "code":  "INVALID_AUTH_FORMAT",
            })
            return
        }

        tokenString := parts[1]
        claims, err := utils.ValidateToken(tokenString)
        if err != nil {
            log.Printf("[AUTH] 🔴 Ошибка: невалидный токен для %s %s: %v", method, path, err)
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid or expired access token",
                "code":  "INVALID_TOKEN",
            })
            return
        }

        // Успешная авторизация
        c.Set("userID", claims.UserID)
        c.Set("role", claims.Role)
        log.Printf("[AUTH] 🟢 Успешная авторизация: %s %s, user=%s, role=%s", 
            method, path, claims.UserID, claims.Role)
        
        c.Next()
    }
}

// AdminMiddleware проверяет роль admin
func AdminMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        // В режиме разработки пропускаем всех
        if cfg.SkipAuth {
            log.Printf("[ADMIN] 🟢 Режим разработки: доступ разрешен для %s %s", method, path)
            c.Next()
            return
        }

        role, exists := c.Get("role")
        if !exists {
            log.Printf("[ADMIN] 🔴 Ошибка: роль не найдена для %s %s", method, path)
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "unauthorized - role not found",
                "code":  "ROLE_NOT_FOUND",
            })
            return
        }

        if role != "admin" {
            log.Printf("[ADMIN] 🔴 Ошибка: доступ запрещен для роли %v на %s %s", role, method, path)
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "admin access required",
                "code":  "ADMIN_REQUIRED",
            })
            return
        }

        log.Printf("[ADMIN] 🟢 Доступ разрешен для admin на %s %s", method, path)
        c.Next()
    }
}