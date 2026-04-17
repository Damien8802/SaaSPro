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

type ReconciliationItem struct {
    Date           string
    DocumentNumber string
    DocumentType   string
    Debit          float64
    Credit         float64
}

func GenerateReconciliationAct(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    userID := c.GetString("user_id")
    
    var req ReconciliationRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    periodStart, err := time.Parse("2006-01-02", req.PeriodStart)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат даты начала"})
        return
    }
    
    periodEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат даты окончания"})
        return
    }
    
    var items []ReconciliationItem
    var totalDebit, totalCredit float64
    
    // Пробуем получить данные из journal_entries
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT operation_date, document_number, document_type, debit_amount, credit_amount
        FROM journal_entries
        WHERE tenant_id = $1 
          AND operation_date BETWEEN $2 AND $3
          AND (counterparty_name = $4 OR counterparty_inn = $5)
        ORDER BY operation_date
    `, tenantID, periodStart, periodEnd, req.CounterpartyName, req.CounterpartyINN)
    
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var date time.Time
            var docNumber, docType string
            var debit, credit float64
            if err := rows.Scan(&date, &docNumber, &docType, &debit, &credit); err == nil {
                items = append(items, ReconciliationItem{
                    Date:           date.Format("02.01.2006"),
                    DocumentNumber: docNumber,
                    DocumentType:   docType,
                    Debit:          debit,
                    Credit:         credit,
                })
                totalDebit += debit
                totalCredit += credit
            }
        }
    }
    
    closingBalance := totalDebit - totalCredit
    
    // Сохраняем акт
    actID := uuid.New()
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO reconciliation_acts (id, tenant_id, counterparty_name, counterparty_inn,
            period_start, period_end, total_debit, total_credit, closing_balance, status, created_by, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'generated', $10, NOW())
    `, actID, tenantID, req.CounterpartyName, req.CounterpartyINN,
        periodStart, periodEnd, totalDebit, totalCredit, closingBalance, userID)
    
    // Генерируем PDF
    pdf := generatePDF(req.CounterpartyName, req.CounterpartyINN, periodStart, periodEnd, items, totalDebit, totalCredit, closingBalance)
    
    c.Header("Content-Type", "application/pdf")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=act_sverki_%s_%s.pdf", 
        req.CounterpartyName, time.Now().Format("20060102_150405")))
    c.Data(http.StatusOK, "application/pdf", pdf.Bytes())
}

func generatePDF(counterpartyName, counterpartyINN string, periodStart, periodEnd time.Time, 
    items []ReconciliationItem, totalDebit, totalCredit, closingBalance float64) bytes.Buffer {
    
    var buf bytes.Buffer
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    
    // Заголовок
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(190, 10, "АКТ СВЕРКИ ВЗАИМОРАСЧЁТОВ")
    pdf.Ln(12)
    
    pdf.SetFont("Arial", "", 12)
    pdf.Cell(190, 8, fmt.Sprintf("Контрагент: %s", counterpartyName))
    pdf.Ln(8)
    if counterpartyINN != "" {
        pdf.Cell(190, 8, fmt.Sprintf("ИНН: %s", counterpartyINN))
        pdf.Ln(8)
    }
    pdf.Cell(190, 8, fmt.Sprintf("Период: %s - %s", periodStart.Format("02.01.2006"), periodEnd.Format("02.01.2006")))
    pdf.Ln(8)
    pdf.Cell(190, 8, fmt.Sprintf("Дата формирования: %s", time.Now().Format("02.01.2006 15:04:05")))
    pdf.Ln(15)
    
    // Таблица
    pdf.SetFont("Arial", "B", 10)
    pdf.Cell(35, 10, "Дата")
    pdf.Cell(45, 10, "Номер документа")
    pdf.Cell(50, 10, "Тип документа")
    pdf.Cell(30, 10, "Дебет")
    pdf.Cell(30, 10, "Кредит")
    pdf.Ln(10)
    
    pdf.SetFont("Arial", "", 10)
    if len(items) == 0 {
        pdf.Cell(35, 8, "—")
        pdf.Cell(45, 8, "—")
        pdf.Cell(50, 8, "Нет операций за период")
        pdf.Cell(30, 8, "0.00")
        pdf.Cell(30, 8, "0.00")
        pdf.Ln(8)
    } else {
        for _, item := range items {
            pdf.Cell(35, 8, item.Date)
            pdf.Cell(45, 8, item.DocumentNumber)
            pdf.Cell(50, 8, item.DocumentType)
            pdf.Cell(30, 8, fmt.Sprintf("%.2f", item.Debit))
            pdf.Cell(30, 8, fmt.Sprintf("%.2f", item.Credit))
            pdf.Ln(8)
        }
    }
    
    // Итоги
    pdf.SetFont("Arial", "B", 10)
    pdf.Ln(5)
    pdf.Cell(130, 10, "ИТОГО:")
    pdf.Cell(30, 10, fmt.Sprintf("%.2f", totalDebit))
    pdf.Cell(30, 10, fmt.Sprintf("%.2f", totalCredit))
    pdf.Ln(10)
    
    pdf.Cell(190, 10, fmt.Sprintf("Сальдо на конец периода: %.2f руб.", closingBalance))
    pdf.Ln(20)
    
    // Подписи
    pdf.SetFont("Arial", "", 10)
    pdf.Cell(95, 10, "От организации: __________________")
    pdf.Cell(95, 10, "От контрагента: __________________")
    
    pdf.Output(&buf)
    return buf
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
