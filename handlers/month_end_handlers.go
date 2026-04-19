package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "subscription-system/database"
)

// MonthEndClosing - структура закрытия месяца
type MonthEndClosing struct {
    ID                uuid.UUID  `json:"id"`
    TenantID          uuid.UUID  `json:"tenant_id"`
    Month             int        `json:"month"`
    Year              int        `json:"year"`
    Status            string     `json:"status"`
    DepreciationAmount float64   `json:"depreciation_amount"`
    CostWriteOff      float64    `json:"cost_write_off"`
    TaxAmount         float64    `json:"tax_amount"`
    NetProfit         float64    `json:"net_profit"`
    StartedAt         time.Time  `json:"started_at"`
    CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

// StartMonthEndClosing - начать процедуру закрытия месяца
func StartMonthEndClosing(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Month int `json:"month" binding:"required"`
        Year  int `json:"year" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    closingID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO month_end_closing (id, tenant_id, month, year, status, started_at)
        VALUES ($1, $2, $3, $4, 'in_progress', NOW())
    `, closingID, tenantID, req.Month, req.Year)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":    "Закрытие месяца запущено",
        "closing_id": closingID,
        "status":     "in_progress",
    })
}

// GetMonthEndStatus - получить статус закрытия месяца
func GetMonthEndStatus(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    month := c.Query("month")
    year := c.Query("year")

    var closing MonthEndClosing
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, month, year, status, COALESCE(depreciation_amount, 0), COALESCE(cost_write_off, 0), 
               COALESCE(tax_amount, 0), COALESCE(net_profit, 0), started_at, completed_at
        FROM month_end_closing
        WHERE tenant_id = $1 AND month = $2 AND year = $3
        ORDER BY started_at DESC LIMIT 1
    `, tenantID, month, year).Scan(
        &closing.ID, &closing.Month, &closing.Year, &closing.Status,
        &closing.DepreciationAmount, &closing.CostWriteOff, &closing.TaxAmount, &closing.NetProfit,
        &closing.StartedAt, &closing.CompletedAt,
    )

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Закрытие месяца не найдено"})
        return
    }

    c.JSON(http.StatusOK, closing)
}

// GetMonthEndHistory - история закрытий месяцев
func GetMonthEndHistory(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT month, year, status, COALESCE(depreciation_amount, 0), COALESCE(cost_write_off, 0), 
               COALESCE(tax_amount, 0), COALESCE(net_profit, 0), started_at, completed_at
        FROM month_end_closing
        WHERE tenant_id = $1
        ORDER BY year DESC, month DESC
        LIMIT 12
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var history []gin.H
    for rows.Next() {
        var month, year int
        var status string
        var depreciation, costWriteOff, tax, profit float64
        var startedAt, completedAt *time.Time

        rows.Scan(&month, &year, &status, &depreciation, &costWriteOff, &tax, &profit, &startedAt, &completedAt)

        history = append(history, gin.H{
            "month":          month,
            "year":           year,
            "status":         status,
            "depreciation":   depreciation,
            "cost_write_off": costWriteOff,
            "tax_amount":     tax,
            "net_profit":     profit,
            "started_at":     startedAt,
            "completed_at":   completedAt,
        })
    }

    c.JSON(http.StatusOK, history)
}

// CreateMonthEndTables - создание таблиц для закрытия месяца
func CreateMonthEndTables(c *gin.Context) {
    _, err := database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS month_end_closing (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            month INTEGER NOT NULL,
            year INTEGER NOT NULL,
            status VARCHAR(50) DEFAULT 'draft',
            depreciation_amount DECIMAL(15,2) DEFAULT 0,
            cost_write_off DECIMAL(15,2) DEFAULT 0,
            tax_amount DECIMAL(15,2) DEFAULT 0,
            net_profit DECIMAL(15,2) DEFAULT 0,
            started_at TIMESTAMP,
            completed_at TIMESTAMP,
            created_at TIMESTAMP DEFAULT NOW(),
            UNIQUE(tenant_id, month, year)
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Таблицы для закрытия месяца созданы",
        "tables":  []string{"month_end_closing"},
    })
}