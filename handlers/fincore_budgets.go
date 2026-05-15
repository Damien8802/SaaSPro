package handlers

import (
    "net/http"
    "strconv"
    "time"
    
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// GetBudgets - получить бюджеты для тега
func GetBudgets(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    tagID := c.Query("tag_id")
    year := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT tag_id, year, month, planned_amount, actual_amount
        FROM fincore_budgets
        WHERE tenant_id = $1 AND year = $2
        AND ($3 = '' OR tag_id = $3::uuid)
        ORDER BY month
    `, tenantID, year, tagID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var budgets []gin.H
    for rows.Next() {
        var tagID, year, month string
        var planned, actual float64
        
        rows.Scan(&tagID, &year, &month, &planned, &actual)
        
        budgets = append(budgets, gin.H{
            "tag_id":   tagID,
            "year":     year,
            "month":    month,
            "planned":  planned,
            "actual":   actual,
            "variance": actual - planned,
            "percent": func() float64 {
                if planned == 0 {
                    return 0
                }
                return (actual / planned) * 100
            }(),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "data":    budgets,
    })
}

// UpdateBudget - обновить бюджет
func UpdateBudget(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    var req struct {
        TagID   string  `json:"tag_id"`
        Year    int     `json:"year"`
        Month   int     `json:"month"`
        Planned float64 `json:"planned"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO fincore_budgets (tenant_id, tag_id, year, month, planned_amount, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        ON CONFLICT (tenant_id, tag_id, year, month) 
        DO UPDATE SET planned_amount = EXCLUDED.planned_amount, updated_at = NOW()
    `, tenantID, req.TagID, req.Year, req.Month, req.Planned)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Бюджет сохранён"})
}