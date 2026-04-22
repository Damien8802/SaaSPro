package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type WorkflowEngine struct {
    db *pgxpool.Pool
}

func NewWorkflowEngine(db *pgxpool.Pool) *WorkflowEngine {
    return &WorkflowEngine{db: db}
}

func (we *WorkflowEngine) ExecuteWorkflows(tenantID, triggerEvent string, actionResult map[string]interface{}) []string {
    var results []string
    
    rows, err := we.db.Query(context.Background(),
        `SELECT id, name, actions FROM ai_workflows 
         WHERE tenant_id = $1 AND trigger_event = $2 AND is_active = true`,
        tenantID, triggerEvent)
    if err != nil {
        return results
    }
    defer rows.Close()
    
    for rows.Next() {
        var id, name string
        var actionsJSON []byte
        rows.Scan(&id, &name, &actionsJSON)
        
        var actions []map[string]interface{}
        if err := json.Unmarshal(actionsJSON, &actions); err != nil {
            continue
        }
        
        for _, action := range actions {
            actionType := action["action"].(string)
            params := action["params"].(map[string]interface{})
            
            switch actionType {
            case "create_task":
                title := params["title"].(string)
                if customerName, ok := actionResult["name"]; ok {
                    title = fmt.Sprintf("%s для %v", title, customerName)
                }
                
                taskID := uuid.New().String()
                _, err := we.db.Exec(context.Background(),
                    `INSERT INTO tasks (id, title, priority, tenant_id, status, created_at) 
                     VALUES ($1, $2, $3, $4, $5, $6)`,
                    taskID, title, "high", tenantID, "pending", time.Now())
                if err == nil {
                    results = append(results, fmt.Sprintf("✅ Задача '%s' создана", title))
                }
                
            case "send_notification":
                message := params["message"].(string)
                results = append(results, fmt.Sprintf("📧 Уведомление: %s", message))
            }
        }
        
        // Обновляем updated_at
        we.db.Exec(context.Background(),
            `UPDATE ai_workflows SET updated_at = NOW() WHERE id = $1`, id)
    }
    
    return results
}

func (we *WorkflowEngine) GetWorkflows(tenantID string) ([]map[string]interface{}, error) {
    rows, err := we.db.Query(context.Background(),
        `SELECT id, name, trigger_event, actions, is_active, created_at 
         FROM ai_workflows WHERE tenant_id = $1 ORDER BY created_at DESC`,
        tenantID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var workflows []map[string]interface{}
    for rows.Next() {
        var id, name, triggerEvent string
        var actions []byte
        var isActive bool
        var createdAt time.Time
        
        rows.Scan(&id, &name, &triggerEvent, &actions, &isActive, &createdAt)
        
        workflows = append(workflows, map[string]interface{}{
            "id":           id,
            "name":         name,
            "trigger_event": triggerEvent,
            "actions":      json.RawMessage(actions),
            "is_active":    isActive,
            "created_at":   createdAt,
        })
    }
    return workflows, nil
}

func (we *WorkflowEngine) CreateWorkflow(tenantID, name, triggerEvent string, actions json.RawMessage) error {
    id := uuid.New().String()
    _, err := we.db.Exec(context.Background(),
        `INSERT INTO ai_workflows (id, tenant_id, name, trigger_event, actions, is_active, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())`,
        id, tenantID, name, triggerEvent, actions)
    return err
}

func (we *WorkflowEngine) GetRecommendations(tenantID string) ([]map[string]interface{}, error) {
    rows, err := we.db.Query(context.Background(),
        `SELECT id, type, title, description, suggested_action, priority, status, created_at
         FROM ai_recommendations WHERE tenant_id = $1 AND status = 'pending'
         ORDER BY priority ASC, created_at DESC LIMIT 20`,
        tenantID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var recommendations []map[string]interface{}
    for rows.Next() {
        var id, rtype, title, description, suggestedAction, status string
        var priority int
        var createdAt time.Time
        
        rows.Scan(&id, &rtype, &title, &description, &suggestedAction, &priority, &status, &createdAt)
        
        recommendations = append(recommendations, map[string]interface{}{
            "id":               id,
            "type":             rtype,
            "title":            title,
            "description":      description,
            "suggested_action": suggestedAction,
            "priority":         priority,
            "status":           status,
            "created_at":       createdAt,
        })
    }
    return recommendations, nil
}