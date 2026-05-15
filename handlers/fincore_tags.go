package handlers

import (
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// GetFincoreTags - получить все теги
func GetFincoreTags(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, type, color, is_active, created_at
        FROM fincore_analytics_tags
        WHERE tenant_id = $1 AND is_active = true
        ORDER BY type, name
    `, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var tags []gin.H
    for rows.Next() {
        var id, name, tagType, color string
        var isActive bool
        var createdAt time.Time
        
        rows.Scan(&id, &name, &tagType, &color, &isActive, &createdAt)
        
        tags = append(tags, gin.H{
            "id":         id,
            "name":       name,
            "type":       tagType,
            "color":      color,
            "is_active":  isActive,
            "created_at": createdAt,
        })
    }
    
    if tags == nil {
        tags = []gin.H{}
    }
    
    c.JSON(http.StatusOK, tags)
}

// CreateFincoreTag - создать тег
func CreateFincoreTag(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    var req struct {
        Name  string `json:"name"`
        Type  string `json:"type"`
        Color string `json:"color"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.Name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Название тега обязательно"})
        return
    }
    
    if req.Type == "" {
        req.Type = "custom"
    }
    
    if req.Color == "" {
        req.Color = "#667eea"
    }
    
    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO fincore_analytics_tags (tenant_id, name, type, color, created_at)
        VALUES ($1, $2, $3, $4, NOW())
        RETURNING id
    `, tenantID, req.Name, req.Type, req.Color).Scan(&id)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "id":      id.String(),
        "message": "Тег успешно создан",
    })
}

// UpdateFincoreTag - обновить тег
func UpdateFincoreTag(c *gin.Context) {
    tagID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    var req struct {
        Name     string `json:"name"`
        Color    string `json:"color"`
        IsActive bool   `json:"is_active"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE fincore_analytics_tags
        SET name = COALESCE(NULLIF($1, ''), name),
            color = COALESCE(NULLIF($2, ''), color),
            is_active = $3,
            updated_at = NOW()
        WHERE id = $4 AND tenant_id = $5
    `, req.Name, req.Color, req.IsActive, tagID, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Тег обновлён"})
}

// DeleteFincoreTag - удалить тег
func DeleteFincoreTag(c *gin.Context) {
    tagID := c.Param("id")
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM fincore_analytics_tags
        WHERE id = $1 AND tenant_id = $2
    `, tagID, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    rowsAffected := result.RowsAffected()
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Тег не найден"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Тег удалён"})
}

// GetFincoreReportByTag - отчёт по тегам
func GetFincoreReportByTag(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    tagID := c.Query("tag_id")
    tagType := c.Query("tag_type")
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    query := `
        SELECT 
            t.id as tag_id,
            t.name as tag_name,
            t.type as tag_type,
            t.color as tag_color,
            COALESCE(SUM(j.debit_amount), 0) as total_debit,
            COALESCE(SUM(j.credit_amount), 0) as total_credit,
            COALESCE(SUM(j.debit_amount - j.credit_amount), 0) as balance
        FROM fincore_analytics_tags t
        LEFT JOIN journal_entry_tags jet ON jet.tag_id = t.id
        LEFT JOIN journal_entries j ON jet.entry_id = j.id AND j.operation_date BETWEEN $2 AND $3
        WHERE t.tenant_id = $1 AND t.is_active = true
    `
    
    args := []interface{}{tenantID, startDate, endDate}
    argIndex := 4
    
    if tagID != "" {
        query += " AND t.id = $" + string(rune(argIndex))
        args = append(args, tagID)
        argIndex++
    }
    if tagType != "" {
        query += " AND t.type = $" + string(rune(argIndex))
        args = append(args, tagType)
        argIndex++
    }
    
    query += " GROUP BY t.id, t.name, t.type, t.color ORDER BY balance DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var results []gin.H
    for rows.Next() {
        var tagID, tagName, tagType, tagColor string
        var totalDebit, totalCredit, balance float64
        
        rows.Scan(&tagID, &tagName, &tagType, &tagColor, &totalDebit, &totalCredit, &balance)
        
        results = append(results, gin.H{
            "tag_id":       tagID,
            "tag_name":     tagName,
            "tag_type":     tagType,
            "tag_color":    tagColor,
            "total_debit":  totalDebit,
            "total_credit": totalCredit,
            "balance":      balance,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "start_date": startDate,
        "end_date":   endDate,
        "data":       results,
    })
}

// AssignTagToEntry - привязать тег к проводке
func AssignTagToEntry(c *gin.Context) {
    var req struct {
        EntryID string `json:"entry_id"`
        TagID   string `json:"tag_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO journal_entry_tags (entry_id, tag_id)
        VALUES ($1, $2)
        ON CONFLICT DO NOTHING
    `, req.EntryID, req.TagID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Тег привязан к проводке"})
}

// RemoveTagFromEntry - отвязать тег от проводки
func RemoveTagFromEntry(c *gin.Context) {
    entryID := c.Param("entry_id")
    tagID := c.Param("tag_id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM journal_entry_tags
        WHERE entry_id = $1 AND tag_id = $2
    `, entryID, tagID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Тег отвязан от проводки"})
}

// ExportFincoreReport - экспорт отчёта в Excel
func ExportFincoreReport(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT 
            t.name as tag_name,
            t.type as tag_type,
            COALESCE(SUM(j.debit_amount), 0) as total_debit,
            COALESCE(SUM(j.credit_amount), 0) as total_credit,
            COALESCE(SUM(j.debit_amount - j.credit_amount), 0) as balance
        FROM fincore_analytics_tags t
        LEFT JOIN journal_entry_tags jet ON jet.tag_id = t.id
        LEFT JOIN journal_entries j ON jet.entry_id = j.id AND j.operation_date BETWEEN $2 AND $3
        WHERE t.tenant_id = $1 AND t.is_active = true
        GROUP BY t.id, t.name, t.type
        ORDER BY balance DESC
    `, tenantID, startDate, endDate)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    html := `<html><head><meta charset="UTF-8"><title>Управленческий отчёт</title></head><body>
    <h2>Отчёт по аналитике</h2>
    <p>Период: ` + startDate + ` - ` + endDate + `</p>
    <table border="1">
        <thead><tr><th>Тег</th><th>Тип</th><th>Дебет</th><th>Кредит</th><th>Сальдо</th></tr></thead><tbody>`
    
    for rows.Next() {
        var name, tagType string
        var debit, credit, balance float64
        rows.Scan(&name, &tagType, &debit, &credit, &balance)
        html += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td align='right'>%.2f</td><td align='right'>%.2f</td><td align='right'>%.2f</td></tr>", 
            name, tagType, debit, credit, balance)
    }
    
    html += `</tbody></table><p>Сформировано: ` + time.Now().Format("2006-01-02 15:04:05") + `</p></body></html>`
    
    filename := fmt.Sprintf("fincore_report_%s_%s.xls", startDate, endDate)
    c.Header("Content-Type", "application/vnd.ms-excel")
    c.Header("Content-Disposition", "attachment; filename="+filename)
    c.String(http.StatusOK, html)
}

// GetTopTags - топ тегов по прибыли
func GetTopTags(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    limit := c.DefaultQuery("limit", "5")
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT 
            t.name,
            t.color,
            COALESCE(SUM(j.debit_amount - j.credit_amount), 0) as balance
        FROM fincore_analytics_tags t
        LEFT JOIN journal_entry_tags jet ON jet.tag_id = t.id
        LEFT JOIN journal_entries j ON jet.entry_id = j.id AND j.operation_date BETWEEN $2 AND $3
        WHERE t.tenant_id = $1 AND t.is_active = true
        GROUP BY t.id, t.name, t.color
        ORDER BY balance DESC
        LIMIT $4
    `, tenantID, startDate, endDate, limit)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var tags []gin.H
    for rows.Next() {
        var name, color string
        var balance float64
        rows.Scan(&name, &color, &balance)
        tags = append(tags, gin.H{
            "name":    name,
            "color":   color,
            "balance": balance,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "data":    tags,
    })
}