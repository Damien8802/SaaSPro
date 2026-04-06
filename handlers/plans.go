package handlers

import (
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

type Plan struct {
    ID             string  `json:"id"`
    Name           string  `json:"name"`
    Description    string  `json:"description"`
    DealsLimit     int     `json:"deals_limit"`
    CustomersLimit int     `json:"customers_limit"`
    StorageLimitMB int     `json:"storage_limit_mb"`
    Price          float64 `json:"price"`
    Features       map[string]interface{} `json:"features"`
}

// GetPlans возвращает список тарифов
func GetPlans(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, description, deals_limit, customers_limit, 
               storage_limit_mb, price, features
        FROM plans WHERE is_active = true ORDER BY price
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    plans := []Plan{}
    for rows.Next() {
        var p Plan
        rows.Scan(&p.ID, &p.Name, &p.Description, &p.DealsLimit, 
                  &p.CustomersLimit, &p.StorageLimitMB, &p.Price, &p.Features)
        plans = append(plans, p)
    }
    c.JSON(http.StatusOK, plans)
}

// GetCurrentUserPlan возвращает текущий тариф пользователя
func GetCurrentUserPlan(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    var plan Plan
    var expiresAt *string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT p.id, p.name, p.description, p.deals_limit, p.customers_limit,
               p.storage_limit_mb, p.price, p.features, u.plan_expires_at
        FROM users u
        JOIN plans p ON p.id = u.plan_id
        WHERE u.id = $1
    `, userID).Scan(&plan.ID, &plan.Name, &plan.Description, &plan.DealsLimit,
        &plan.CustomersLimit, &plan.StorageLimitMB, &plan.Price, &plan.Features, &expiresAt)
    
    if err != nil {
        // Если нет плана - назначаем бесплатный
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT id, name, description, deals_limit, customers_limit,
                   storage_limit_mb, price, features
            FROM plans WHERE name = 'Free'
        `).Scan(&plan.ID, &plan.Name, &plan.Description, &plan.DealsLimit,
            &plan.CustomersLimit, &plan.StorageLimitMB, &plan.Price, &plan.Features)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "plan": plan,
        "expires_at": expiresAt,
    })
}

// GetPlansHandler возвращает список тарифов
func GetPlansHandler(c *gin.Context) {
    c.JSON(http.StatusOK, []map[string]interface{}{
        {"id": "free", "name": "Бесплатный", "price": 0, "deals_limit": 100},
        {"id": "pro", "name": "Профессиональный", "price": 2990, "deals_limit": 1000},
        {"id": "business", "name": "Бизнес", "price": 9990, "deals_limit": 10000},
    })
}