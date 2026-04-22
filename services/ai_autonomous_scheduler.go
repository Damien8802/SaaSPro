package services

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

func StartAutonomousScheduler(db *pgxpool.Pool) {
    ticker := time.NewTicker(5 * time.Minute)
    log.Println("🤖 Autonomous AI Scheduler started (every 5 min)")
    
    for range ticker.C {
        runAutonomousChecks(db)
    }
}

func runAutonomousChecks(db *pgxpool.Pool) {
    // Проверка просроченных счетов
    checkOverdueInvoices(db)
    
    // Анализ активности клиентов
    analyzeCustomerActivity(db)
    
    log.Println("🔄 Autonomous AI checks completed")
}

func checkOverdueInvoices(db *pgxpool.Pool) {
    rows, err := db.Query(context.Background(),
        `SELECT id, number, customer_name, amount, due_date, tenant_id
         FROM invoices 
         WHERE status = 'sent' AND due_date < NOW() AND due_date > NOW() - INTERVAL '30 days'`)
    if err != nil {
        return
    }
    defer rows.Close()
    
    for rows.Next() {
        var id, number, customerName, tenantID string
        var amount float64
        var dueDate time.Time
        
        rows.Scan(&id, &number, &customerName, &amount, &dueDate, &tenantID)
        
        daysOverdue := int(time.Since(dueDate).Hours() / 24)
        
        // Создаём рекомендацию
        db.Exec(context.Background(),
            `INSERT INTO ai_recommendations (id, tenant_id, type, title, description, suggested_action, priority, status, created_at)
             VALUES (gen_random_uuid(), $1, 'overdue_invoice', $2, $3, 'send_reminder', 1, 'pending', NOW())`,
            tenantID,
            "Просрочен счёт",
            fmt.Sprintf("Счёт №%s для %s просрочен на %d дней. Сумма: %.2f ₽", number, customerName, daysOverdue, amount))
    }
}

func analyzeCustomerActivity(db *pgxpool.Pool) {
    // Анализ клиентов без активности 30+ дней
    rows, err := db.Query(context.Background(),
        `SELECT DISTINCT c.id, c.name, c.tenant_id
         FROM customers c
         LEFT JOIN deals d ON d.customer_id = c.id AND d.created_at > NOW() - INTERVAL '30 days'
         WHERE d.id IS NULL AND c.created_at < NOW() - INTERVAL '30 days'`)
    if err != nil {
        return
    }
    defer rows.Close()
    
    for rows.Next() {
        var id, name, tenantID string
        rows.Scan(&id, &name, &tenantID)
        
        db.Exec(context.Background(),
            `INSERT INTO ai_recommendations (id, tenant_id, type, title, description, suggested_action, priority, status, created_at)
             VALUES (gen_random_uuid(), $1, 'inactive_customer', $2, $3, 'create_task', 2, 'pending', NOW())`,
            tenantID,
            "Неактивный клиент",
            fmt.Sprintf("Клиент '%s' не создавал сделок более 30 дней. Рекомендуется связаться.", name))
    }
}