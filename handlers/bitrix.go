package handlers

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// BitrixSettings структура настроек
type BitrixSettings struct {
    ID          uuid.UUID       `json:"id"`
    WebhookURL  string          `json:"webhook_url"`
    Domain      string          `json:"domain"`
    MemberID    string          `json:"member_id"`
    AccessToken string          `json:"access_token"`
    LastSync    *time.Time      `json:"last_sync"`
    SyncStatus  string          `json:"sync_status"`
    Settings    json.RawMessage `json:"settings"`
}

// GetBitrixSettings - получить настройки Bitrix
func GetBitrixSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var settings BitrixSettings
    var lastSync sql.NullTime
    var settingsJSON []byte
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, webhook_url, domain, member_id, access_token, 
               last_sync, sync_status, settings
        FROM bitrix_settings
        WHERE user_id = $1
    `, userID).Scan(
        &settings.ID, &settings.WebhookURL, &settings.Domain, &settings.MemberID,
        &settings.AccessToken, &lastSync, &settings.SyncStatus, &settingsJSON,
    )
    
    if err != nil {
        // Настройки по умолчанию
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "settings": map[string]interface{}{
                "webhook_url":   "",
                "domain":        "",
                "sync_status":   "idle",
                "auto_sync":     false,
                "sync_interval": 3600,
            },
        })
        return
    }
    
    if lastSync.Valid {
        settings.LastSync = &lastSync.Time
    }
    
    var settingsMap map[string]interface{}
    if len(settingsJSON) > 0 {
        json.Unmarshal(settingsJSON, &settingsMap)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "settings": settings,
        "extra":    settingsMap,
    })
}

// SaveBitrixSettings - сохранить настройки Bitrix
func SaveBitrixSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        WebhookURL string          `json:"webhook_url"`
        Domain     string          `json:"domain"`
        MemberID   string          `json:"member_id"`
        Settings   json.RawMessage `json:"settings"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO bitrix_settings (user_id, webhook_url, domain, member_id, settings, sync_status, updated_at)
        VALUES ($1, $2, $3, $4, $5, 'idle', NOW())
        ON CONFLICT (user_id) DO UPDATE SET
            webhook_url = EXCLUDED.webhook_url,
            domain = EXCLUDED.domain,
            member_id = EXCLUDED.member_id,
            settings = EXCLUDED.settings,
            updated_at = NOW()
    `, userID, req.WebhookURL, req.Domain, req.MemberID, req.Settings)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить настройки"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки сохранены",
    })
}

// ExportLeadToBitrix - экспорт лида в Bitrix24
func ExportLeadToBitrix(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Title       string  `json:"title" binding:"required"`
        Name        string  `json:"name"`
        Phone       string  `json:"phone"`
        Email       string  `json:"email"`
        Description string  `json:"description"`
        Price       float64 `json:"price"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Получаем настройки Bitrix
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    // Формируем данные для Bitrix
    leadData := map[string]interface{}{
        "fields": map[string]interface{}{
            "TITLE":       req.Title,
            "NAME":        req.Name,
            "COMMENTS":    req.Description,
            "OPPORTUNITY": req.Price,
            "CURRENCY_ID": "RUB",
        },
    }
    
    // Добавляем телефон если есть
    if req.Phone != "" {
        leadData["fields"].(map[string]interface{})["PHONE"] = []map[string]string{
            {"VALUE": req.Phone, "VALUE_TYPE": "WORK"},
        }
    }
    
    // Добавляем email если есть
    if req.Email != "" {
        leadData["fields"].(map[string]interface{})["EMAIL"] = []map[string]string{
            {"VALUE": req.Email, "VALUE_TYPE": "WORK"},
        }
    }
    
    jsonData, _ := json.Marshal(leadData)
    
    // Отправляем в Bitrix
    resp, err := http.Post(webhookURL+"/crm.lead.add", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка отправки в Bitrix"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    // Сохраняем лог
    logID := uuid.New()
    bitrixID := ""
    if id, ok := result["result"]; ok {
        bitrixID = fmt.Sprintf("%v", id)
    }
    
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, bitrix_id, action, status, response, created_at)
        VALUES ($1, $2, 'export', 'lead', $3, 'create', 'completed', $4, NOW())
    `, logID, userID, bitrixID, string(jsonData))
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "bitrix_id": bitrixID,
        "message":   "Лид экспортирован в Bitrix24",
    })
}

// ImportLeadsFromBitrix - импорт лидов из Bitrix24
func ImportLeadsFromBitrix(c *gin.Context) {
    userID := getUserID(c)
    
    // Получаем настройки
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    // Запрашиваем лиды из Bitrix
    resp, err := http.Get(webhookURL + "/crm.lead.list?select[]=ID&select[]=TITLE&select[]=NAME&select[]=PHONE&select[]=EMAIL&select[]=COMMENTS")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения лидов из Bitrix"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    leads, ok := result["result"].([]interface{})
    if !ok {
        c.JSON(http.StatusOK, gin.H{
            "success":  true,
            "imported": 0,
            "message":  "Нет новых лидов",
        })
        return
    }
    
    imported := 0
    for _, lead := range leads {
        leadMap, ok := lead.(map[string]interface{})
        if !ok {
            continue
        }
        
        name := ""
        if n, ok := leadMap["NAME"]; ok {
            name = fmt.Sprintf("%v", n)
        }
        
        email := ""
        if e, ok := leadMap["EMAIL"]; ok {
            email = fmt.Sprintf("%v", e)
        }
        
        phone := ""
        if p, ok := leadMap["PHONE"]; ok {
            phone = fmt.Sprintf("%v", p)
        }
        
        // Сохраняем в нашу CRM
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO crm_customers (user_id, name, email, phone, lead_score, created_at)
            VALUES ($1, $2, $3, $4, 50, NOW())
            ON CONFLICT (email) DO NOTHING
        `, userID, name, email, phone)
        
        if err == nil {
            imported++
        }
        
        // Сохраняем лог
        logID := uuid.New()
        bitrixID := fmt.Sprintf("%v", leadMap["ID"])
        database.Pool.Exec(c.Request.Context(), `
            INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, bitrix_id, action, status, created_at)
            VALUES ($1, $2, 'import', 'lead', $3, 'create', 'completed', NOW())
        `, logID, userID, bitrixID)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "imported": imported,
        "message":  fmt.Sprintf("Импортировано %d лидов", imported),
    })
}

// GetBitrixSyncLogs - получить логи синхронизации
func GetBitrixSyncLogs(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, direction, entity_type, bitrix_id, action, status, error_message, created_at, synced_at
        FROM bitrix_sync_logs
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var logs []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var direction, entityType, bitrixID, action, status, errorMsg string
        var createdAt, syncedAt time.Time
        var syncedAtPtr *time.Time
        
        rows.Scan(&id, &direction, &entityType, &bitrixID, &action, &status, &errorMsg, &createdAt, &syncedAt)
        
        if !syncedAt.IsZero() {
            syncedAtPtr = &syncedAt
        }
        
        logs = append(logs, map[string]interface{}{
            "id":          id,
            "direction":   direction,
            "entity_type": entityType,
            "bitrix_id":   bitrixID,
            "action":      action,
            "status":      status,
            "error":       errorMsg,
            "created_at":  createdAt,
            "synced_at":   syncedAtPtr,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "logs":    logs,
    })
}

// SyncBitrixContacts - синхронизация контактов
func SyncBitrixContacts(c *gin.Context) {
    userID := getUserID(c)
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    // Добавляем колонки если нет
    _, err = database.Pool.Exec(c.Request.Context(), `
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_synced BOOLEAN DEFAULT false;
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_synced_at TIMESTAMP;
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_id VARCHAR(100);
    `)
    if err != nil {
        // Игнорируем ошибки
    }
    
    // Получаем контакты из нашей CRM
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, email, phone, company
        FROM crm_customers
        WHERE user_id = $1 AND (bitrix_synced = false OR bitrix_synced IS NULL)
        LIMIT 100
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var synced int
    for rows.Next() {
        var id uuid.UUID
        var name, email, phone, company string
        
        rows.Scan(&id, &name, &email, &phone, &company)
        
        contactData := map[string]interface{}{
            "fields": map[string]interface{}{
                "NAME":         name,
                "COMPANY_TITLE": company,
            },
        }
        
        if email != "" {
            contactData["fields"].(map[string]interface{})["EMAIL"] = []map[string]string{
                {"VALUE": email, "VALUE_TYPE": "WORK"},
            }
        }
        
        if phone != "" {
            contactData["fields"].(map[string]interface{})["PHONE"] = []map[string]string{
                {"VALUE": phone, "VALUE_TYPE": "WORK"},
            }
        }
        
        jsonData, _ := json.Marshal(contactData)
        resp, err := http.Post(webhookURL+"/crm.contact.add", "application/json", bytes.NewBuffer(jsonData))
        if err == nil {
            resp.Body.Close()
            synced++
            
            // Обновляем статус синхронизации
            database.Pool.Exec(c.Request.Context(), `
                UPDATE crm_customers SET bitrix_synced = true, bitrix_synced_at = NOW()
                WHERE id = $1
            `, id)
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "synced":  synced,
        "message": fmt.Sprintf("Синхронизировано %d контактов", synced),
    })
}