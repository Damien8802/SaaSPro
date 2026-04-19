package handlers

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "subscription-system/database"
)

// GetEmployeesForPayroll - список сотрудников для расчёта зарплаты
func GetEmployeesForPayroll(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, position, 
               COALESCE(salary, 0) as salary, 
               COALESCE(tax_rate, 13) as tax_rate
        FROM hr_employees 
        WHERE tenant_id = $1 AND status = 'active'
        ORDER BY last_name
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки сотрудников: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load employees"})
        return
    }
    defer rows.Close()
    
    var employees []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName, position string
        var salary, taxRate float64
        
        err := rows.Scan(&id, &firstName, &lastName, &position, &salary, &taxRate)
        if err != nil {
            log.Printf("⚠️ Ошибка сканирования: %v", err)
            continue
        }
        
        tax := salary * taxRate / 100
        netAmount := salary - tax
        
        employees = append(employees, gin.H{
            "id":         id,
            "name":       firstName + " " + lastName,
            "position":   position,
            "salary":     salary,
            "tax_rate":   taxRate,
            "tax":        tax,
            "net_amount": netAmount,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"employees": employees})
}

// CalculatePayroll - расчёт зарплаты за период
func CalculatePayroll(c *gin.Context) {
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
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, salary, tax_rate
        FROM hr_employees 
        WHERE tenant_id = $1 AND status = 'active'
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки сотрудников: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load employees"})
        return
    }
    defer rows.Close()
    
    var payrolls []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName string
        var salary, taxRate float64
        
        rows.Scan(&id, &firstName, &lastName, &salary, &taxRate)
        
        tax := salary * taxRate / 100
        netAmount := salary - tax
        
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO payroll (id, tenant_id, employee_id, period_month, period_year, salary, tax, net_amount, status, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'calculated', NOW())
            ON CONFLICT (employee_id, period_month, period_year) DO UPDATE
            SET salary = $6, tax = $7, net_amount = $8, status = 'calculated'
        `, uuid.New(), tenantID, id, req.Month, req.Year, salary, tax, netAmount)
        
        if err != nil {
            log.Printf("⚠️ Ошибка вставки payroll: %v", err)
        }
        
        payrolls = append(payrolls, gin.H{
            "employee_id": id,
            "name":        firstName + " " + lastName,
            "salary":      salary,
            "tax":         tax,
            "net_amount":  netAmount,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":  "Расчёт выполнен",
        "payrolls": payrolls,
        "total":    len(payrolls),
        "month":    req.Month,
        "year":     req.Year,
    })
}

// GetPayrollHistory - история начислений (исправленная версия)
func GetPayrollHistory(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, e.first_name, e.last_name, p.period_month, p.period_year, 
               p.salary, p.tax, p.net_amount, p.status, p.created_at
        FROM payroll p
        JOIN hr_employees e ON p.employee_id = e.id
        WHERE p.tenant_id = $1
        ORDER BY p.period_year DESC, p.period_month DESC, e.last_name
        LIMIT 100
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки истории: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load history"})
        return
    }
    defer rows.Close()
    
    var history []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName string
        var month, year int
        var salary, tax, netAmount float64
        var status string
        var createdAt time.Time
        
        err := rows.Scan(&id, &firstName, &lastName, &month, &year, &salary, &tax, &netAmount, &status, &createdAt)
        if err != nil {
            log.Printf("⚠️ Ошибка сканирования: %v", err)
            continue
        }
        
        history = append(history, gin.H{
            "id":         id,
            "employee":   firstName + " " + lastName,
            "period":     fmt.Sprintf("%d/%d", month, year),
            "salary":     salary,
            "tax":        tax,
            "net_amount": netAmount,
            "status":     status,
            "created_at": createdAt.Format("2006-01-02"),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"history": history})
}

// ProcessPayrollPayment - выплата зарплаты
func ProcessPayrollPayment(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        PayrollID string `json:"payroll_id" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE payroll 
        SET status = 'paid', paid_at = NOW()
        WHERE id = $1 AND tenant_id = $2
    `, req.PayrollID, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка выплаты: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Зарплата выплачена"})
}

// GenerateTaxReport - генерация налогового отчёта
func GenerateTaxReport(c *gin.Context) {
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
    
    // Собираем данные за период
    var totalIncome, totalTax float64
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT COALESCE(SUM(salary), 0), COALESCE(SUM(tax), 0)
        FROM payroll
        WHERE tenant_id = $1 AND period_month = $2 AND period_year = $3
    `, tenantID, req.Month, req.Year)
    
    if err == nil {
        defer rows.Close()
        if rows.Next() {
            rows.Scan(&totalIncome, &totalTax)
        }
    }
    
    // Сохраняем отчёт
    reportID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO tax_reports (id, tenant_id, report_type, period_month, period_year, total_income, total_tax, created_at)
        VALUES ($1, $2, '6-НДФЛ', $3, $4, $5, $6, NOW())
    `, reportID, tenantID, req.Month, req.Year, totalIncome, totalTax)
    
    if err != nil {
        log.Printf("❌ Ошибка генерации отчёта: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":      "Отчёт сгенерирован",
        "report_id":    reportID,
        "total_income": totalIncome,
        "total_tax":    totalTax,
        "month":        req.Month,
        "year":         req.Year,
    })
}

// ========== ДОБАВЬТЕ ЭТИ ФУНКЦИИ В КОНЕЦ ФАЙЛА payroll.go ==========

// CalculateSickLeave - расчёт больничного листа
func CalculateSickLeave(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        EmployeeID string    `json:"employee_id" binding:"required"`
        StartDate  time.Time `json:"start_date" binding:"required"`
        EndDate    time.Time `json:"end_date" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Получаем данные сотрудника
    var salary, experienceYears float64
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(salary, 0), COALESCE(EXTRACT(YEAR FROM AGE(NOW(), hire_date)), 0)
        FROM hr_employees 
        WHERE id = $1 AND tenant_id = $2
    `, req.EmployeeID, tenantID).Scan(&salary, &experienceYears)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found"})
        return
    }
    
    // Количество дней больничного
    daysCount := int(req.EndDate.Sub(req.StartDate).Hours() / 24) + 1
    
    // Процент оплаты в зависимости от стажа
    var payPercent float64
    if experienceYears < 5 {
        payPercent = 0.60
    } else if experienceYears < 8 {
        payPercent = 0.80
    } else {
        payPercent = 1.00
    }
    
    // Среднедневной заработок
    avgDailySalary := salary / 29.3
    
    // Сумма больничного
    amount := avgDailySalary * float64(daysCount) * payPercent
    
    // Сохраняем
    sickLeaveID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO sick_leaves (id, tenant_id, employee_id, start_date, end_date, days_count, amount, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'approved', NOW())
    `, sickLeaveID, tenantID, req.EmployeeID, req.StartDate, req.EndDate, daysCount, amount)
    
    if err != nil {
        log.Printf("❌ Ошибка сохранения больничного: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save sick leave"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":        "Больничный рассчитан",
        "sick_leave_id":  sickLeaveID,
        "days_count":     daysCount,
        "pay_percent":    payPercent * 100,
        "amount":         amount,
        "experience_years": experienceYears,
    })
}

// CalculateVacation - расчёт отпускных
func CalculateVacation(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        EmployeeID string    `json:"employee_id" binding:"required"`
        StartDate  time.Time `json:"start_date" binding:"required"`
        DaysCount  int       `json:"days_count" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Получаем зарплату за последние 12 месяцев
    var avgSalary float64
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(AVG(salary), 0)
        FROM payroll
        WHERE employee_id = $1 AND tenant_id = $2 
        AND period_year >= EXTRACT(YEAR FROM NOW()) - 1
    `, req.EmployeeID, tenantID).Scan(&avgSalary)
    
    if err != nil || avgSalary == 0 {
        // Если нет истории, берём текущую зарплату
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT COALESCE(salary, 0) FROM hr_employees WHERE id = $1
        `, req.EmployeeID).Scan(&avgSalary)
    }
    
    // Среднедневной заработок
    avgDailySalary := avgSalary / 29.3
    
    // Сумма отпускных
    amount := avgDailySalary * float64(req.DaysCount)
    
    // Сохраняем
    vacationID := uuid.New()
    endDate := req.StartDate.AddDate(0, 0, req.DaysCount-1)
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO vacations (id, tenant_id, employee_id, start_date, end_date, days_count, amount, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'approved', NOW())
    `, vacationID, tenantID, req.EmployeeID, req.StartDate, endDate, req.DaysCount, amount)
    
    if err != nil {
        log.Printf("❌ Ошибка сохранения отпуска: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save vacation"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":      "Отпускные рассчитаны",
        "vacation_id":  vacationID,
        "days_count":   req.DaysCount,
        "avg_salary":   avgSalary,
        "amount":       amount,
        "end_date":     endDate,
    })
}

// CalculateAlimony - расчёт алиментов
func CalculateAlimony(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        EmployeeID   string  `json:"employee_id" binding:"required"`
        ChildrenCount int    `json:"children_count" binding:"required"`
        NetSalary    float64 `json:"net_salary"` // если не передана, рассчитаем сами
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Если не передана чистая зарплата, получаем из БД
    netSalary := req.NetSalary
    if netSalary == 0 {
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT COALESCE(net_amount, 0)
            FROM payroll
            WHERE employee_id = $1 AND tenant_id = $2
            ORDER BY period_year DESC, period_month DESC
            LIMIT 1
        `, req.EmployeeID, tenantID).Scan(&netSalary)
    }
    
    // Процент алиментов в зависимости от количества детей
    var percent float64
    switch req.ChildrenCount {
    case 1:
        percent = 0.25 // 25% на одного ребёнка
    case 2:
        percent = 0.33 // 33% на двоих детей
    default:
        percent = 0.50 // 50% на трёх и более
    }
    
    alimonyAmount := netSalary * percent
    
    c.JSON(http.StatusOK, gin.H{
        "employee_id":    req.EmployeeID,
        "children_count": req.ChildrenCount,
        "net_salary":     netSalary,
        "percent":        percent * 100,
        "alimony_amount": alimonyAmount,
        "message":        fmt.Sprintf("Алименты составят %.2f руб. (%.0f%%)", alimonyAmount, percent*100),
    })
}

// GeneratePaymentOrder - формирование платёжной ведомости
func GeneratePaymentOrder(c *gin.Context) {
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
    
    // Получаем все начисления за период
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, e.first_name, e.last_name, e.position, 
               p.salary, p.tax, p.net_amount,
               COALESCE((SELECT amount FROM sick_leaves WHERE employee_id = e.id AND status = 'approved' 
                         AND EXTRACT(YEAR FROM start_date) = $2 AND EXTRACT(MONTH FROM start_date) = $1), 0) as sick_pay,
               COALESCE((SELECT amount FROM vacations WHERE employee_id = e.id AND status = 'approved'
                         AND EXTRACT(YEAR FROM start_date) = $2 AND EXTRACT(MONTH FROM start_date) = $1), 0) as vacation_pay
        FROM payroll p
        JOIN hr_employees e ON p.employee_id = e.id
        WHERE p.tenant_id = $3 AND p.period_month = $1 AND p.period_year = $2
    `, req.Month, req.Year, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var employees []gin.H
    var totalNetAmount float64
    
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName, position string
        var salary, tax, netAmount, sickPay, vacationPay float64
        
        rows.Scan(&id, &firstName, &lastName, &position, &salary, &tax, &netAmount, &sickPay, &vacationPay)
        
        totalWithAdditions := netAmount + sickPay + vacationPay
        totalNetAmount += totalWithAdditions
        
        employees = append(employees, gin.H{
            "id":           id,
            "name":         firstName + " " + lastName,
            "position":     position,
            "salary":       salary,
            "sick_pay":     sickPay,
            "vacation_pay": vacationPay,
            "tax":          tax,
            "net_amount":   totalWithAdditions,
        })
    }
    
    // Формируем PDF (в данном случае JSON, но можно расширить до PDF)
    paymentOrder := gin.H{
        "order_number":   fmt.Sprintf("ВП-%d%02d", req.Year, req.Month),
        "date":           time.Now().Format("2006-01-02"),
        "month":          req.Month,
        "year":           req.Year,
        "total_amount":   totalNetAmount,
        "employees_count": len(employees),
        "employees":      employees,
        "status":         "ready_for_payment",
    }
    
    c.JSON(http.StatusOK, paymentOrder)
}

// GetEmployeePayrollDetails - детализация зарплаты сотрудника
func GetEmployeePayrollDetails(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    employeeID := c.Param("id")
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.period_month, p.period_year, p.salary, p.tax, p.net_amount, p.status, p.paid_at,
               COALESCE((SELECT SUM(amount) FROM sick_leaves WHERE employee_id = p.employee_id 
                         AND EXTRACT(YEAR FROM start_date) = p.period_year 
                         AND EXTRACT(MONTH FROM start_date) = p.period_month), 0) as sick_pay,
               COALESCE((SELECT SUM(amount) FROM vacations WHERE employee_id = p.employee_id
                         AND EXTRACT(YEAR FROM start_date) = p.period_year
                         AND EXTRACT(MONTH FROM start_date) = p.period_month), 0) as vacation_pay
        FROM payroll p
        WHERE p.employee_id = $1 AND p.tenant_id = $2
        ORDER BY p.period_year DESC, p.period_month DESC
    `, employeeID, tenantID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var history []gin.H
    for rows.Next() {
        var month, year int
        var salary, tax, netAmount, sickPay, vacationPay float64
        var status string
        var paidAt *time.Time
        
        rows.Scan(&month, &year, &salary, &tax, &netAmount, &status, &paidAt, &sickPay, &vacationPay)
        
        totalNet := netAmount + sickPay + vacationPay
        
        history = append(history, gin.H{
            "period":       fmt.Sprintf("%d/%d", month, year),
            "salary":       salary,
            "sick_pay":     sickPay,
            "vacation_pay": vacationPay,
            "tax":          tax,
            "net_amount":   totalNet,
            "status":       status,
            "paid_at":      paidAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "employee_id": employeeID,
        "history":     history,
    })
}

// CreatePayrollTables - создание таблиц для ЗУП
func CreatePayrollTables(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    // Таблица больничных
    _, err := database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS sick_leaves (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            start_date DATE NOT NULL,
            end_date DATE NOT NULL,
            days_count INTEGER DEFAULT 0,
            amount DECIMAL(15,2) DEFAULT 0,
            status VARCHAR(50) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW(),
            approved_at TIMESTAMP
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Таблица отпусков
    _, err = database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS vacations (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            start_date DATE NOT NULL,
            end_date DATE NOT NULL,
            days_count INTEGER DEFAULT 0,
            amount DECIMAL(15,2) DEFAULT 0,
            status VARCHAR(50) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW(),
            approved_at TIMESTAMP
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Таблица алиментов
    _, err = database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS alimonies (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            employee_id UUID NOT NULL,
            children_count INTEGER DEFAULT 0,
            percent DECIMAL(5,2) DEFAULT 0,
            amount DECIMAL(15,2) DEFAULT 0,
            month INTEGER,
            year INTEGER,
            created_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "Таблицы ЗУП созданы",
        "tables":  []string{"sick_leaves", "vacations", "alimonies"},
    })
}