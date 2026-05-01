package middleware

import (
    "context"
    "log"  
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// Список email'ов владельцев системы (безлимитный доступ)
var ownerEmails = map[string]bool{
    "dev@saaspro.ru":          true,
    "Skorpion_88-88@mail.ru": true,
}

// isOwner - проверяет, является ли пользователь владельцем системы
func isOwner(email string) bool {
    return ownerEmails[email]
}

// ========== МОДУЛИ (VPN, CRM, IDENTITY HUB) ==========

// RequireModuleAccess - проверяет доступ к модулю
func RequireModuleAccess(moduleName string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("role")
        email := c.GetString("user_email")

        // ВЛАДЕЛЕЦ СИСТЕМЫ - БЕЗЛИМИТНЫЙ ДОСТУП
        if isOwner(email) {
            c.Next()
            return
        }

        // РАЗРАБОТЧИК И АДМИН - БЕЗЛИМИТНЫЙ ДОСТУП
        if role == "developer" || role == "admin" {
            c.Next()
            return
        }

        userID := c.GetString("user_id")
        tenantID := c.GetString("tenant_id")

        if tenantID == "" {
            c.JSON(http.StatusForbidden, gin.H{
                "error":   "Доступ запрещён",
                "message": "Tenant ID не найден",
            })
            c.Abort()
            return
        }

        // 1. Проверяем активную подписку
        var expiresAt time.Time
        var subStatus string
        err := database.Pool.QueryRow(context.Background(), `
            SELECT status, expires_at FROM user_subscriptions 
            WHERE user_id = $1 AND module_name = $2 AND status = 'active'
            ORDER BY expires_at DESC LIMIT 1
        `, userID, moduleName).Scan(&subStatus, &expiresAt)

        if err == nil && expiresAt.After(time.Now()) {
            c.Next()
            return
        }

        // 2. Проверяем триальный период
        var trialEnd time.Time
        err = database.Pool.QueryRow(context.Background(), `
            SELECT trial_end FROM user_trials 
            WHERE user_id = $1 AND module_name = $2 AND trial_end > NOW()
        `, userID, moduleName).Scan(&trialEnd)

        if err == nil && trialEnd.After(time.Now()) {
            daysLeft := int(time.Until(trialEnd).Hours() / 24)
            if daysLeft == 3 || daysLeft == 1 {
                var notified bool
                database.Pool.QueryRow(context.Background(), `
                    SELECT notified FROM user_trials 
                    WHERE user_id = $1 AND module_name = $2
                `, userID, moduleName).Scan(&notified)

                if !notified {
                    fmt.Printf("🔔 Уведомление: У пользователя %s заканчивается триал модуля %s через %d дней\n", userID, moduleName, daysLeft)
                    database.Pool.Exec(context.Background(), `
                        UPDATE user_trials SET notified = true
                        WHERE user_id = $1 AND module_name = $2
                    `, userID, moduleName)
                }
            }
            c.Next()
            return
        }

        // 3. Нет доступа
        c.JSON(http.StatusPaymentRequired, gin.H{
            "error":            "Модуль не оплачен",
            "message":          "Для доступа к этому модулю необходимо оплатить подписку или начать 14-дневный пробный период",
            "module":           moduleName,
            "trial_available":  true,
            "trial_days":       14,
            "upgrade_url":      "/pricing",
            "start_trial_url":  "/api/trial/start?module=" + moduleName,
        })
        c.Abort()
    }
}

// StartModuleTrial - начать триальный период для пользователя
func StartModuleTrial(userID, moduleName string) error {
    log.Printf("🔍 StartModuleTrial called: userID=%s, moduleName=%s", userID, moduleName)
    
    _, err := database.Pool.Exec(context.Background(), `
        INSERT INTO user_trials (user_id, module_name, trial_start, trial_end, notified)
        VALUES ($1, $2, NOW(), NOW() + INTERVAL '14 days', false)
        ON CONFLICT (user_id, module_name) DO UPDATE SET
            trial_start = NOW(),
            trial_end = NOW() + INTERVAL '14 days',
            notified = false
    `, userID, moduleName)
    
    if err != nil {
        log.Printf("❌ Exec error: %v", err)
        return err
    }
    
    log.Printf("✅ Trial activated successfully for user %s, module %s", userID, moduleName)
    return nil
}
// GetAvailableModules - возвращает список доступных модулей
func GetAvailableModules(tenantID string) []string {
    if tenantID == "" {
        return []string{}
    }

    rows, err := database.Pool.Query(context.Background(), `
        SELECT module_name FROM user_subscriptions
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

// ========== РАЗРАБОТЧИКИ (DEVELOPER PORTAL) ==========

// RequireDeveloperAccess - проверяет доступ к Developer Portal
func RequireDeveloperAccess() gin.HandlerFunc {
    return func(c *gin.Context) {
        role := c.GetString("role")
        userID := c.GetString("user_id")
        email := c.GetString("user_email")

        // ВЛАДЕЛЕЦ СИСТЕМЫ - БЕЗЛИМИТНЫЙ ДОСТУП
        if isOwner(email) {
            c.Next()
            return
        }

        // Администратор имеет полный доступ
        if role == "admin" {
            c.Next()
            return
        }

        // Разработчик (по роли) имеет доступ
        if role == "developer" {
            c.Next()
            return
        }

        // Проверяем подписку разработчика
        var plan string
        var trialEnd time.Time
        var subscriptionEnd time.Time
        var status string

        err := database.Pool.QueryRow(context.Background(), `
            SELECT plan, trial_end, subscription_end, status
            FROM developer_subscriptions
            WHERE user_id = $1
        `, userID).Scan(&plan, &trialEnd, &subscriptionEnd, &status)

        if err == nil && status == "active" {
            // Проверяем триал
            if plan == "trial" && trialEnd.After(time.Now()) {
                c.Next()
                return
            }
            // Проверяем платную подписку
            if (plan == "pro" || plan == "enterprise") && subscriptionEnd.After(time.Now()) {
                c.Next()
                return
            }
        }

        // Нет доступа — предлагаем купить подписку
        c.JSON(http.StatusPaymentRequired, gin.H{
            "error":       "Доступ к Developer Portal требует подписки",
            "message":     "Станьте разработчиком, чтобы создавать OAuth-приложения",
            "trial_days":  14,
            "plans": []gin.H{
                {"name": "Пробный", "price": 0, "days": 14, "apps": 1, "users": 100},
                {"name": "Pro", "price": 2990, "apps": 10, "users": 10000},
                {"name": "Enterprise", "price": 14990, "apps": -1, "users": -1},
            },
            "upgrade_url": "/developer/pricing",
        })
        c.Abort()
    }
}


// StartDeveloperTrial - начать триальный период для разработчика
func StartDeveloperTrial(userID string) error {
    // Проверяем, нет ли уже активного триала
    var existingEnd time.Time
    err := database.Pool.QueryRow(context.Background(), `
        SELECT trial_end FROM developer_trials 
        WHERE user_id = $1 AND trial_end > NOW()
    `, userID).Scan(&existingEnd)
    
    if err == nil {
        // Триал уже активен, но всё равно повышаем роль на всякий случай
        _, _ = database.Pool.Exec(context.Background(), `
            UPDATE users SET role = 'developer' WHERE id = $1 AND role = 'user'
        `, userID)
        return nil
    }
    
    // Создаём триал на 14 дней
    trialEnd := time.Now().Add(14 * 24 * time.Hour)
    _, err = database.Pool.Exec(context.Background(), `
        INSERT INTO developer_trials (user_id, trial_end, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (user_id) DO UPDATE SET trial_end = $2
    `, userID, trialEnd)
    
    if err != nil {
        return err
    }
    
    // ПОВЫШАЕМ РОЛЬ ДО DEVELOPER (ВОТ ЭТО ВАЖНО!)
    _, err = database.Pool.Exec(context.Background(), `
        UPDATE users SET role = 'developer' WHERE id = $1 AND role = 'user'
    `, userID)
    
    return err
}
// GetDeveloperSubscription - получить информацию о подписке разработчика
func GetDeveloperSubscription(userID string) (map[string]interface{}, error) {
    var plan string
    var trialEnd, subscriptionEnd time.Time
    var maxApps, maxUsers int
    var status string

    err := database.Pool.QueryRow(context.Background(), `
        SELECT plan, trial_end, subscription_end, max_apps, max_users, status
        FROM developer_subscriptions
        WHERE user_id = $1
    `, userID).Scan(&plan, &trialEnd, &subscriptionEnd, &maxApps, &maxUsers, &status)

    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "plan":                       plan,
        "trial_end":                  trialEnd,
        "subscription_end":           subscriptionEnd,
        "max_apps":                   maxApps,
        "max_users":                  maxUsers,
        "status":                     status,
        "is_trial_active":            plan == "trial" && trialEnd.After(time.Now()),
        "is_subscription_active":     (plan == "pro" || plan == "enterprise") && subscriptionEnd.After(time.Now()),
    }, nil
}