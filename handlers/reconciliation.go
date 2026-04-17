package handlers

import (
    "bytes"
    "fmt"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/jung-kurt/gofpdf"
    "subscription-system/database"
    "subscription-system/middleware"
)

type ReconciliationRequest struct {
    CounterpartyName string `json:"counterparty_name" binding:"required"`
    CounterpartyINN  string `json:"counterparty_inn"`
    PeriodStart      string `json:"period_start" binding:"required"`
    PeriodEnd        string `json:"period_end" binding:"required"`
}

func GenerateReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    userID := c.GetString("user_id")
    
    var req ReconciliationRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    periodStart, _ := time.Parse("2006-01-02", req.PeriodStart)
    periodEnd, _ := time.Parse("2006-01-02", req.PeriodEnd)
    
    actID := uuid.New()
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO reconciliation_acts (id, tenant_id, counterparty_name, counterparty_inn,
            period_start, period_end, total_debit, total_credit, closing_balance, status, created_by, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, 0, 0, 0, 'generated', $7, NOW())
    `, actID, tenantID, req.CounterpartyName, req.CounterpartyINN,
        periodStart, periodEnd, userID)
    
    var buf bytes.Buffer
    pdf := gofpdf.New("P", "mm", "A4", "")
    
    // Регистрируем русский шрифт Roboto
    pdf.AddUTF8Font("Roboto", "", "fonts/Roboto-Regular.ttf")
    pdf.AddUTF8Font("Roboto", "B", "fonts/Roboto-Bold.ttf")
    
    pdf.AddPage()
    
    // Логотип
    pdf.SetFont("Roboto", "B", 20)
    pdf.SetTextColor(79, 70, 229)
    pdf.Cell(190, 15, "SaaSPro")
    pdf.SetTextColor(0, 0, 0)
    pdf.Ln(10)
    
    // Заголовок
    pdf.SetFont("Roboto", "B", 18)
    pdf.Cell(190, 10, "АКТ СВЕРКИ ВЗАИМОРАСЧЁТОВ")
    pdf.Ln(15)
    
    // Информация о контрагенте
    pdf.SetFont("Roboto", "", 11)
    pdf.Cell(190, 8, "Контрагент: "+req.CounterpartyName)
    pdf.Ln(8)
    if req.CounterpartyINN != "" {
        pdf.Cell(190, 8, "ИНН: "+req.CounterpartyINN)
        pdf.Ln(8)
    }
    pdf.Cell(190, 8, "Период: "+periodStart.Format("02.01.2006")+" - "+periodEnd.Format("02.01.2006"))
    pdf.Ln(8)
    pdf.Cell(190, 8, "Дата формирования: "+time.Now().Format("02.01.2006 15:04:05"))
    pdf.Ln(15)
    
    // Таблица
    pdf.SetFont("Roboto", "B", 10)
    pdf.Cell(30, 10, "Дата")
    pdf.Cell(40, 10, "Номер документа")
    pdf.Cell(50, 10, "Тип документа")
    pdf.Cell(35, 10, "Дебет, руб.")
    pdf.Cell(35, 10, "Кредит, руб.")
    pdf.Ln(10)
    
    pdf.SetFont("Roboto", "", 9)
    pdf.Cell(30, 8, "-")
    pdf.Cell(40, 8, "-")
    pdf.Cell(50, 8, "Нет операций")
    pdf.Cell(35, 8, "0.00")
    pdf.Cell(35, 8, "0.00")
    pdf.Ln(8)
    
    // Итоги
    pdf.SetFont("Roboto", "B", 10)
    pdf.Ln(5)
    pdf.Cell(120, 10, "ИТОГО:")
    pdf.Cell(35, 10, "0.00")
    pdf.Cell(35, 10, "0.00")
    pdf.Ln(12)
    
    pdf.Cell(190, 10, "Сальдо на конец периода: 0.00 руб.")
    pdf.Ln(20)
    
    // Подписи
    pdf.SetFont("Roboto", "", 10)
    pdf.Cell(95, 10, "От организации: __________________")
    pdf.Cell(95, 10, "От контрагента: __________________")
    pdf.Ln(8)
    pdf.Cell(95, 10, "(должность, ФИО, подпись)")
    pdf.Cell(95, 10, "(должность, ФИО, подпись)")
    
    pdf.Output(&buf)
    
    c.Header("Content-Type", "application/pdf")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=act_sverki_%s.pdf", time.Now().Format("20060102_150405")))
    c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}

func GetReconciliationActs(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, counterparty_name, counterparty_inn, period_start, period_end,
               total_debit, total_credit, closing_balance, status, created_at
        FROM reconciliation_acts
        WHERE tenant_id = $1
        ORDER BY created_at DESC
    `, tenantID)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"acts": []gin.H{}})
        return
    }
    defer rows.Close()
    
    var acts []gin.H
    for rows.Next() {
        var id uuid.UUID
        var counterpartyName, counterpartyINN, status string
        var periodStart, periodEnd time.Time
        var totalDebit, totalCredit, closingBalance float64
        var createdAt time.Time
        
        rows.Scan(&id, &counterpartyName, &counterpartyINN, &periodStart, &periodEnd,
            &totalDebit, &totalCredit, &closingBalance, &status, &createdAt)
        
        acts = append(acts, gin.H{
            "id":                id,
            "counterparty_name": counterpartyName,
            "counterparty_inn":  counterpartyINN,
            "period_start":      periodStart.Format("02.01.2006"),
            "period_end":        periodEnd.Format("02.01.2006"),
            "total_debit":       totalDebit,
            "total_credit":      totalCredit,
            "closing_balance":   closingBalance,
            "status":            status,
            "created_at":        createdAt.Format("02.01.2006 15:04"),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"acts": acts})
}

func GetReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    actID := c.Param("id")
    
    var id uuid.UUID
    var counterpartyName, counterpartyINN, status string
    var periodStart, periodEnd time.Time
    var totalDebit, totalCredit, closingBalance float64
    var createdAt time.Time
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, counterparty_name, counterparty_inn, period_start, period_end,
               total_debit, total_credit, closing_balance, status, created_at
        FROM reconciliation_acts
        WHERE id = $1 AND tenant_id = $2
    `, actID, tenantID).Scan(&id, &counterpartyName, &counterpartyINN,
        &periodStart, &periodEnd, &totalDebit, &totalCredit,
        &closingBalance, &status, &createdAt)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Акт не найден"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "id":                id,
        "counterparty_name": counterpartyName,
        "counterparty_inn":  counterpartyINN,
        "period_start":      periodStart.Format("02.01.2006"),
        "period_end":        periodEnd.Format("02.01.2006"),
        "total_debit":       totalDebit,
        "total_credit":      totalCredit,
        "closing_balance":   closingBalance,
        "status":            status,
        "created_at":        createdAt.Format("02.01.2006 15:04"),
    })
}

func DeleteReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    actID := c.Param("id")
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM reconciliation_acts
        WHERE id = $1 AND tenant_id = $2
    `, actID, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    if result.RowsAffected() == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Акт не найден"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Акт удалён"})
}

func UpdateReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    actID := c.Param("id")
    
    var req struct {
        CounterpartyName string `json:"counterparty_name"`
        CounterpartyINN  string `json:"counterparty_inn"`
        PeriodStart      string `json:"period_start"`
        PeriodEnd        string `json:"period_end"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    periodStart, _ := time.Parse("2006-01-02", req.PeriodStart)
    periodEnd, _ := time.Parse("2006-01-02", req.PeriodEnd)
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE reconciliation_acts
        SET counterparty_name = $1,
            counterparty_inn = $2,
            period_start = $3,
            period_end = $4,
            updated_at = NOW()
        WHERE id = $5 AND tenant_id = $6
    `, req.CounterpartyName, req.CounterpartyINN, periodStart, periodEnd, actID, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    if result.RowsAffected() == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Акт не найден"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Акт обновлён"})
}

func DownloadReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    actID := c.Param("id")
    
    var counterpartyName, counterpartyINN string
    var periodStart, periodEnd time.Time
    var totalDebit, totalCredit, closingBalance float64
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT counterparty_name, counterparty_inn, period_start, period_end,
               total_debit, total_credit, closing_balance
        FROM reconciliation_acts
        WHERE id = $1 AND tenant_id = $2
    `, actID, tenantID).Scan(&counterpartyName, &counterpartyINN,
        &periodStart, &periodEnd, &totalDebit, &totalCredit, &closingBalance)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Акт не найден"})
        return
    }
    
    var buf bytes.Buffer
    pdf := gofpdf.New("P", "mm", "A4", "")
    
    pdf.AddUTF8Font("Roboto", "", "fonts/Roboto-Regular.ttf")
    pdf.AddUTF8Font("Roboto", "B", "fonts/Roboto-Bold.ttf")
    
    pdf.AddPage()
    
    pdf.SetFont("Roboto", "B", 20)
    pdf.SetTextColor(79, 70, 229)
    pdf.Cell(190, 15, "SaaSPro")
    pdf.SetTextColor(0, 0, 0)
    pdf.Ln(10)
    
    pdf.SetFont("Roboto", "B", 18)
    pdf.Cell(190, 10, "АКТ СВЕРКИ ВЗАИМОРАСЧЁТОВ")
    pdf.Ln(15)
    
    pdf.SetFont("Roboto", "", 11)
    pdf.Cell(190, 8, "Контрагент: "+counterpartyName)
    pdf.Ln(8)
    if counterpartyINN != "" {
        pdf.Cell(190, 8, "ИНН: "+counterpartyINN)
        pdf.Ln(8)
    }
    pdf.Cell(190, 8, "Период: "+periodStart.Format("02.01.2006")+" - "+periodEnd.Format("02.01.2006"))
    pdf.Ln(8)
    pdf.Cell(190, 8, "Дата формирования: "+time.Now().Format("02.01.2006 15:04:05"))
    pdf.Ln(15)
    
    pdf.SetFont("Roboto", "B", 10)
    pdf.Cell(30, 10, "Дата")
    pdf.Cell(40, 10, "Номер документа")
    pdf.Cell(50, 10, "Тип документа")
    pdf.Cell(35, 10, "Дебет, руб.")
    pdf.Cell(35, 10, "Кредит, руб.")
    pdf.Ln(10)
    
    pdf.SetFont("Roboto", "", 9)
    pdf.Cell(30, 8, "-")
    pdf.Cell(40, 8, "-")
    pdf.Cell(50, 8, "Нет операций")
    pdf.Cell(35, 8, "0.00")
    pdf.Cell(35, 8, "0.00")
    pdf.Ln(8)
    
    pdf.SetFont("Roboto", "B", 10)
    pdf.Ln(5)
    pdf.Cell(120, 10, "ИТОГО:")
    pdf.Cell(35, 10, fmt.Sprintf("%.2f", totalDebit))
    pdf.Cell(35, 10, fmt.Sprintf("%.2f", totalCredit))
    pdf.Ln(12)
    
    pdf.Cell(190, 10, fmt.Sprintf("Сальдо на конец периода: %.2f руб.", closingBalance))
    pdf.Ln(20)
    
    pdf.SetFont("Roboto", "", 10)
    pdf.Cell(95, 10, "От организации: __________________")
    pdf.Cell(95, 10, "От контрагента: __________________")
    pdf.Ln(8)
    pdf.Cell(95, 10, "(должность, ФИО, подпись)")
    pdf.Cell(95, 10, "(должность, ФИО, подпись)")
    
    pdf.Output(&buf)
    
    c.Header("Content-Type", "application/pdf")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=act_sverki_%s.pdf", time.Now().Format("20060102_150405")))
    c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}