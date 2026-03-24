package handlers

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// BalanceItem структура строки ОСВ
type BalanceItem struct {
    AccountID     uuid.UUID `json:"account_id"`
    AccountCode   string    `json:"account_code"`
    AccountName   string    `json:"account_name"`
    AccountType   string    `json:"account_type"`
    OpeningDebit  float64   `json:"opening_debit"`
    OpeningCredit float64   `json:"opening_credit"`
    PeriodDebit   float64   `json:"period_debit"`
    PeriodCredit  float64   `json:"period_credit"`
    ClosingDebit  float64   `json:"closing_debit"`
    ClosingCredit float64   `json:"closing_credit"`
}

// GetTurnoverBalanceSheet - Оборотно-сальдовая ведомость (ОСВ)
func GetTurnoverBalanceSheet(c *gin.Context) {
    userID := getUserID(c)
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    accounts, err := getAccounts(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    postings, err := getPostingsByPeriod(userID, startDate, endDate)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    var osv []BalanceItem
    for _, acc := range accounts {
        item := BalanceItem{
            AccountID:   acc.ID,
            AccountCode: acc.Code,
            AccountName: acc.Name,
            AccountType: acc.AccountType,
        }
        
        for _, p := range postings {
            if p.AccountID == acc.ID {
                item.PeriodDebit += p.DebitAmount
                item.PeriodCredit += p.CreditAmount
            }
        }
        
        if acc.AccountType == "active" || acc.AccountType == "active_passive" {
            item.ClosingDebit = item.OpeningDebit + item.PeriodDebit - item.PeriodCredit
        } else {
            item.ClosingCredit = item.OpeningCredit + item.PeriodCredit - item.PeriodDebit
        }
        
        if item.ClosingDebit > 0 || item.ClosingCredit > 0 || item.PeriodDebit > 0 || item.PeriodCredit > 0 {
            osv = append(osv, item)
        }
    }
    
    totals := struct {
        OpeningDebit  float64 `json:"opening_debit"`
        OpeningCredit float64 `json:"opening_credit"`
        PeriodDebit   float64 `json:"period_debit"`
        PeriodCredit  float64 `json:"period_credit"`
        ClosingDebit  float64 `json:"closing_debit"`
        ClosingCredit float64 `json:"closing_credit"`
    }{}
    
    for _, item := range osv {
        totals.OpeningDebit += item.OpeningDebit
        totals.OpeningCredit += item.OpeningCredit
        totals.PeriodDebit += item.PeriodDebit
        totals.PeriodCredit += item.PeriodCredit
        totals.ClosingDebit += item.ClosingDebit
        totals.ClosingCredit += item.ClosingCredit
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "start_date": startDate,
        "end_date":   endDate,
        "data":       osv,
        "totals":     totals,
    })
}

// GetProfitAndLoss - Отчет о прибылях и убытках
func GetProfitAndLoss(c *gin.Context) {
    userID := getUserID(c)
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    var revenue float64
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
        AND p.account_id IN (
            SELECT id FROM chart_of_accounts 
            WHERE user_id = $1 AND code IN ('90', '91')
        )
    `, userID, startDate, endDate).Scan(&revenue)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    var expenses float64
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
        AND p.account_id IN (
            SELECT id FROM chart_of_accounts 
            WHERE user_id = $1 AND code IN ('20', '26', '44', '91')
        )
    `, userID, startDate, endDate).Scan(&expenses)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    profit := revenue - expenses
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "start_date": startDate,
        "end_date":   endDate,
        "revenue":    revenue,
        "expenses":   expenses,
        "profit":     profit,
    })
}

// GetDashboardStats - Статистика для дашборда
func GetDashboardStats(c *gin.Context) {
    userID := getUserID(c)
    
    var revenue float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '90')
    `, userID).Scan(&revenue)
    
    var expenses float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code IN ('20', '26', '44'))
    `, userID).Scan(&expenses)
    
    var entriesCount int
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM journal_entries 
        WHERE user_id = $1 AND entry_status = 'posted'
    `, userID).Scan(&entriesCount)
    
    var bankBalance float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount - credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '51')
    `, userID).Scan(&bankBalance)
    
    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "revenue":       revenue,
        "expenses":      expenses,
        "profit":        revenue - expenses,
        "entries_count": entriesCount,
        "bank_balance":  bankBalance,
    })
}

// GetSalesChart - Данные для графика продаж
func GetSalesChart(c *gin.Context) {
    userID := getUserID(c)
    
    period := c.DefaultQuery("period", "month")
    
    var interval string
    switch period {
    case "quarter":
        interval = "3 months"
    case "year":
        interval = "1 year"
    default:
        interval = "1 month"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT 
            DATE_TRUNC('day', e.entry_date) as date,
            COALESCE(SUM(p.credit_amount), 0) as total
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= NOW() - $2::INTERVAL
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '90')
        GROUP BY DATE_TRUNC('day', e.entry_date)
        ORDER BY date
    `, userID, interval)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var dates []string
    var values []float64
    
    for rows.Next() {
        var date time.Time
        var total float64
        rows.Scan(&date, &total)
        dates = append(dates, date.Format("2006-01-02"))
        values = append(values, total)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "period":  period,
        "labels":  dates,
        "data":    values,
    })
}

// GetSalesByProduct - Анализ продаж по товарам
func GetSalesByProduct(c *gin.Context) {
    userID := getUserID(c)
    
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
            p.name as product_name,
            COALESCE(p.sku, '') as sku,
            SUM(oi.quantity) as quantity_sold,
            SUM(oi.total) as total_amount
        FROM order_items oi
        JOIN orders o ON oi.order_id = o.id
        JOIN products p ON oi.product_id = p.id
        WHERE o.user_id = $1 
        AND o.created_at BETWEEN $2 AND $3
        AND o.status != 'cancelled'
        GROUP BY p.id, p.name, p.sku
        ORDER BY total_amount DESC
        LIMIT 50
    `, userID, startDate, endDate)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var products []map[string]interface{}
    var totalSold int
    var totalRevenue float64
    
    for rows.Next() {
        var name, sku string
        var quantity int
        var amount float64
        rows.Scan(&name, &sku, &quantity, &amount)
        products = append(products, map[string]interface{}{
            "name":     name,
            "sku":      sku,
            "quantity": quantity,
            "amount":   amount,
        })
        totalSold += quantity
        totalRevenue += amount
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "start_date":    startDate,
        "end_date":      endDate,
        "products":      products,
        "total_sold":    totalSold,
        "total_revenue": totalRevenue,
    })
}

// GetFinancialRatios - Финансовые коэффициенты
func GetFinancialRatios(c *gin.Context) {
    userID := getUserID(c)
    
    var revenue float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(p.credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '90')
    `, userID).Scan(&revenue)
    
    var cost float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(p.debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '20')
    `, userID).Scan(&cost)
    
    var expenses float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(p.debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code IN ('26', '44'))
    `, userID).Scan(&expenses)
    
    var assets float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(CASE WHEN a.code IN ('50', '51') THEN 
            p.debit_amount - p.credit_amount ELSE 0 END), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        JOIN chart_of_accounts a ON p.account_id = a.id
        WHERE e.user_id = $1 AND e.entry_status = 'posted'
        AND a.code IN ('50', '51')
    `, userID).Scan(&assets)
    
    var liabilities float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(p.credit_amount - p.debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        JOIN chart_of_accounts a ON p.account_id = a.id
        WHERE e.user_id = $1 AND e.entry_status = 'posted'
        AND a.code = '60'
    `, userID).Scan(&liabilities)
    
    profit := revenue - cost - expenses
    
    safeDiv := func(a, b float64) float64 {
        if b == 0 {
            return 0
        }
        return a / b
    }
    
    ratios := map[string]interface{}{
        "profit_margin":   safeDiv(profit, revenue) * 100,
        "gross_margin":    safeDiv(revenue-cost, revenue) * 100,
        "roe":             safeDiv(profit, assets) * 100,
        "current_ratio":   safeDiv(assets, liabilities),
        "revenue_growth":  0,
        "profit_growth":   0,
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "period":       "month",
        "revenue":      revenue,
        "cost":         cost,
        "expenses":     expenses,
        "profit":       profit,
        "assets":       assets,
        "liabilities":  liabilities,
        "ratios":       ratios,
    })
}

// GetInventoryTurnover - Оборачиваемость товаров
func GetInventoryTurnover(c *gin.Context) {
    userID := getUserID(c)
    
    var sales float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(total_amount), 0)
        FROM orders
        WHERE user_id = $1 AND created_at >= DATE_TRUNC('month', NOW())
        AND status != 'cancelled'
    `, userID).Scan(&sales)
    
    var avgStock float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(AVG(quantity), 0)
        FROM products
        WHERE user_id = $1 AND active = true
    `, userID).Scan(&avgStock)
    
    safeDiv := func(a, b float64) float64 {
        if b == 0 {
            return 0
        }
        return a / b
    }
    
    turnover := safeDiv(sales, avgStock)
    
    c.JSON(http.StatusOK, gin.H{
        "success":        true,
        "sales":          sales,
        "avg_stock":      avgStock,
        "turnover":       turnover,
        "turnover_days":  safeDiv(30, turnover),
    })
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

type accountInfo struct {
    ID          uuid.UUID
    Code        string
    Name        string
    AccountType string
}

type postingInfo struct {
    AccountID    uuid.UUID
    DebitAmount  float64
    CreditAmount float64
}

func getAccounts(userID uuid.UUID) ([]accountInfo, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, code, name, account_type 
        FROM chart_of_accounts 
        WHERE user_id = $1 AND is_active = true
        ORDER BY code
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var accounts []accountInfo
    for rows.Next() {
        var a accountInfo
        rows.Scan(&a.ID, &a.Code, &a.Name, &a.AccountType)
        accounts = append(accounts, a)
    }
    return accounts, nil
}

func getPostingsByPeriod(userID uuid.UUID, startDate, endDate string) ([]postingInfo, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT p.account_id, p.debit_amount, p.credit_amount
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
    `, userID, startDate, endDate)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var postings []postingInfo
    for rows.Next() {
        var p postingInfo
        rows.Scan(&p.AccountID, &p.DebitAmount, &p.CreditAmount)
        postings = append(postings, p)
    }
    return postings, nil
}