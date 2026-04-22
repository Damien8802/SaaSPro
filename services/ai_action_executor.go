package services

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"
    
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type AIActionExecutor struct {
    db *pgxpool.Pool
}

func NewAIActionExecutor(db *pgxpool.Pool) *AIActionExecutor {
    return &AIActionExecutor{db: db}
}

type ActionResult struct {
    Success bool
    Message string
    Data    interface{}
    Error   error
}

func (e *AIActionExecutor) ExecuteAction(tenantID, userID string, intent IntentExtended, entities map[string]string) ActionResult {
    log.Printf("⚡ Executing action: %s", intent.Action)
    
    // Исправляем tenantID
    if tenantID == "" || tenantID == "default" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    switch intent.Action {
    case "create_customer":
        return e.createCustomer(tenantID, entities)
    case "create_deal":
        return e.createDeal(tenantID, entities)
    case "create_invoice":
        return e.createInvoice(tenantID, entities)
    case "create_task":
        return e.createTask(tenantID, entities)
    default:
        return ActionResult{
            Success: true,
            Message: GetHelpMessage(),
        }
    }
}

func (e *AIActionExecutor) createCustomer(tenantID string, entities map[string]string) ActionResult {
    name, ok := entities["name"]
    if !ok || name == "" {
        return ActionResult{Success: false, Message: "❌ Не указано имя клиента"}
    }
    
    customerID := uuid.New().String()
    ctx := context.Background()
    
    // Генерируем уникальный email если не указан
    email := entities["email"]
    if email == "" {
        email = fmt.Sprintf("client_%d@temp.com", time.Now().UnixNano())
    }
    
    phone := entities["phone"]
    if phone == "" {
        phone = ""
    }
    
    _, err := e.db.Exec(ctx, `
        INSERT INTO crm_customers (id, name, phone, email, tenant_id, created_at) 
        VALUES ($1, $2, $3, $4, $5::uuid, NOW())
    `, customerID, name, phone, email, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка создания клиента: %v", err)
        return ActionResult{Success: false, Message: fmt.Sprintf("❌ Ошибка: %v", err), Error: err}
    }
    
    log.Printf("✅ Клиент '%s' создан (ID: %s)", name, customerID)
    
    return ActionResult{
        Success: true,
        Message: fmt.Sprintf("✅ Клиент '%s' создан в CRM!", name),
        Data: map[string]interface{}{"id": customerID, "name": name},
    }
}

func (e *AIActionExecutor) createDeal(tenantID string, entities map[string]string) ActionResult {
    dealName, ok := entities["deal_name"]
    if !ok || dealName == "" {
        return ActionResult{Success: false, Message: "❌ Не указано название сделки"}
    }
    
    amount := entities["amount"]
    if amount == "" {
        amount = "0"
    }
    
    dealID := uuid.New().String()
    ctx := context.Background()
    
    // Используем колонку "value" вместо "amount"
    _, err := e.db.Exec(ctx, `
        INSERT INTO crm_deals (id, title, value, tenant_id, stage, created_at) 
        VALUES ($1, $2, $3, $4::uuid, 'new', NOW())
    `, dealID, dealName, amount, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка создания сделки: %v", err)
        return ActionResult{Success: false, Message: fmt.Sprintf("❌ Ошибка: %v", err), Error: err}
    }
    
    return ActionResult{
        Success: true,
        Message: fmt.Sprintf("✅ Сделка '%s' на сумму %s ₽ создана!", dealName, amount),
        Data: map[string]interface{}{"id": dealID, "title": dealName, "value": amount},
    }
}

func (e *AIActionExecutor) createInvoice(tenantID string, entities map[string]string) ActionResult {
    customerName, ok := entities["customer_name"]
    if !ok || customerName == "" {
        return ActionResult{Success: false, Message: "❌ Не указан клиент"}
    }
    
    amount, ok := entities["amount"]
    if !ok || amount == "" {
        return ActionResult{Success: false, Message: "❌ Не указана сумма"}
    }
    
    invoiceID := uuid.New().String()
    invoiceNumber := fmt.Sprintf("INV-%d", time.Now().UnixNano()%100000)
    ctx := context.Background()
    
    _, err := e.db.Exec(ctx, `
        INSERT INTO invoices (id, number, customer_name, amount, tenant_id, status, created_at, due_date) 
        VALUES ($1, $2, $3, $4, $5::uuid, 'sent', NOW(), NOW() + INTERVAL '14 days')
    `, invoiceID, invoiceNumber, customerName, amount, tenantID)
    
    if err != nil {
        return ActionResult{Success: false, Message: fmt.Sprintf("❌ Ошибка: %v", err), Error: err}
    }
    
    return ActionResult{
        Success: true,
        Message: fmt.Sprintf("✅ Счёт №%s на сумму %s ₽ выставлен клиенту %s!", invoiceNumber, amount, customerName),
        Data: map[string]interface{}{"id": invoiceID, "number": invoiceNumber},
    }
}

func (e *AIActionExecutor) createTask(tenantID string, entities map[string]string) ActionResult {
    title, ok := entities["title"]
    if !ok || title == "" {
        return ActionResult{Success: false, Message: "❌ Не указано название задачи"}
    }
    
    taskID := uuid.New().String()
    assignee := entities["assignee"]
    ctx := context.Background()
    
    // Проверяем, есть ли колонка assignee
    var hasAssignee bool
    e.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT FROM information_schema.columns 
            WHERE table_name = 'tasks' AND column_name = 'assignee'
        )
    `).Scan(&hasAssignee)
    
    var query string
    if hasAssignee {
        query = `INSERT INTO tasks (id, title, assignee, tenant_id, status, created_at) 
                 VALUES ($1, $2, $3, $4::uuid, 'pending', NOW())`
        _, err := e.db.Exec(ctx, query, taskID, title, assignee, tenantID)
        if err != nil {
            return ActionResult{Success: false, Message: fmt.Sprintf("❌ Ошибка: %v", err), Error: err}
        }
    } else {
        query = `INSERT INTO tasks (id, title, tenant_id, status, created_at) 
                 VALUES ($1, $2, $3::uuid, 'pending', NOW())`
        _, err := e.db.Exec(ctx, query, taskID, title, tenantID)
        if err != nil {
            return ActionResult{Success: false, Message: fmt.Sprintf("❌ Ошибка: %v", err), Error: err}
        }
    }
    
    msg := fmt.Sprintf("✅ Задача '%s' создана!", title)
    if assignee != "" && hasAssignee {
        msg = fmt.Sprintf("✅ Задача '%s' создана и назначена %s!", title, assignee)
    }
    
    return ActionResult{
        Success: true,
        Message: msg,
        Data: map[string]interface{}{"id": taskID, "title": title},
    }
}

func (e *AIActionExecutor) SaveActionHistory(tenantID, userID, actionType string, actionData, result interface{}, err error) {
    ctx := context.Background()
    
    var actionDataJSON, resultJSON string
    if actionData != nil {
        data, _ := json.Marshal(actionData)
        actionDataJSON = string(data)
    }
    if result != nil {
        res, _ := json.Marshal(result)
        resultJSON = string(res)
    }
    
    status := "success"
    errorMsg := ""
    if err != nil {
        status = "failed"
        errorMsg = err.Error()
    }
    
    _, dbErr := e.db.Exec(ctx, `
        INSERT INTO ai_action_history (id, tenant_id, user_id, action_type, action_data, result, status, error_message, created_at)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, NOW())
    `, tenantID, userID, actionType, actionDataJSON, resultJSON, status, errorMsg)
    
    if dbErr != nil {
        log.Printf("⚠️ Ошибка сохранения истории: %v", dbErr)
    }
}

func (e *AIActionExecutor) GetActionHistory(tenantID string, limit int) ([]map[string]interface{}, error) {
    ctx := context.Background()
    
    rows, err := e.db.Query(ctx, `
        SELECT id, action_type, action_data, result, status, error_message, created_at
        FROM ai_action_history
        WHERE tenant_id = $1::uuid
        ORDER BY created_at DESC
        LIMIT $2
    `, tenantID, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var history []map[string]interface{}
    for rows.Next() {
        var id, actionType, status, errorMsg string
        var actionData, result []byte
        var createdAt time.Time
        
        rows.Scan(&id, &actionType, &actionData, &result, &status, &errorMsg, &createdAt)
        
        history = append(history, map[string]interface{}{
            "id":          id,
            "action_type": actionType,
            "action_data": string(actionData),
            "result":      string(result),
            "status":      status,
            "error":       errorMsg,
            "created_at":  createdAt,
        })
    }
    return history, nil
}