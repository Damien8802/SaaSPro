package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
    "strconv"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// WhatsAppConfig - конфигурация
type WhatsAppConfig struct {
    AccessToken     string
    PhoneNumberID   string
    BusinessID      string
    VerifyToken     string
}

// WhatsAppMessage - структура сообщения
type WhatsAppMessage struct {
    ID          string    `json:"id"`
    From        string    `json:"from"`
    To          string    `json:"to"`
    Body        string    `json:"body"`
    Type        string    `json:"type"`
    Status      string    `json:"status"`
    MediaURL    string    `json:"media_url"`
    CreatedAt   time.Time `json:"created_at"`
}

// WhatsAppTemplate - шаблон сообщения
type WhatsAppTemplate struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Body        string    `json:"body"`
    Category    string    `json:"category"`
    Variables   []string  `json:"variables"`
    CreatedAt   time.Time `json:"created_at"`
}

// WhatsAppBroadcast - рассылка
type WhatsAppBroadcast struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Message     string     `json:"message"`
    TemplateID  string     `json:"template_id"`
    Recipients  []string   `json:"recipients"`
    Status      string     `json:"status"`
    SentCount   int        `json:"sent_count"`
    FailedCount int        `json:"failed_count"`
    ScheduledAt *time.Time `json:"scheduled_at"`
    CreatedAt   time.Time  `json:"created_at"`
}

// WhatsAppContact - контакт
type WhatsAppContact struct {
    ID          string    `json:"id"`
    PhoneNumber string    `json:"phone_number"`
    Name        string    `json:"name"`
    LastMessage string    `json:"last_message"`
    LastActive  time.Time `json:"last_active"`
    CreatedAt   time.Time `json:"created_at"`
}

// WhatsAppStats - статистика
type WhatsAppStats struct {
    TotalMessages    int     `json:"total_messages"`
    SentMessages     int     `json:"sent_messages"`
    ReceivedMessages int     `json:"received_messages"`
    FailedMessages   int     `json:"failed_messages"`
    TotalTemplates   int     `json:"total_templates"`
    TotalBroadcasts  int     `json:"total_broadcasts"`
    TotalContacts    int     `json:"total_contacts"`
    AvgResponseTime  float64 `json:"avg_response_time"`
}

// ConnectWhatsApp - подключение WhatsApp Business
func ConnectWhatsApp(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        PhoneNumberID   string `json:"phone_number_id" binding:"required"`
        BusinessID      string `json:"business_account_id"`
        AccessToken     string `json:"access_token" binding:"required"`
        VerifyToken     string `json:"verify_token"`
        WebhookURL      string `json:"webhook_url"`
        BusinessName    string `json:"business_name"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Деактивируем старые интеграции
    database.Pool.Exec(c.Request.Context(), `
        UPDATE whatsapp_integrations SET is_active = false WHERE company_id = $1
    `, companyID)

    integrationID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_integrations (id, company_id, phone_number_id, business_account_id, access_token, webhook_verify_token, webhook_url, business_name, is_active, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, NOW())
    `, integrationID, companyID, req.PhoneNumberID, req.BusinessID, req.AccessToken, req.VerifyToken, req.WebhookURL, req.BusinessName)

    if err != nil {
        log.Printf("❌ Ошибка подключения WhatsApp: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect WhatsApp"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "✅ WhatsApp Business подключён успешно!",
        "integration_id": integrationID,
    })
}

// GetWhatsAppStatus - получение статуса интеграции
func GetWhatsAppStatus(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var phoneNumberID, accessToken, businessID, businessName string
    var isActive bool
    var createdAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT phone_number_id, access_token, business_account_id, business_name, is_active, created_at 
        FROM whatsapp_integrations
        WHERE company_id = $1 AND is_active = true
        ORDER BY created_at DESC LIMIT 1
    `, companyID).Scan(&phoneNumberID, &accessToken, &businessID, &businessName, &isActive, &createdAt)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"connected": false})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "connected": isActive,
        "phone_number_id": phoneNumberID,
        "business_id": businessID,
        "business_name": businessName,
        "connected_at": createdAt.Format("2006-01-02 15:04:05"),
    })
}

// DisconnectWhatsApp - отключение WhatsApp
func DisconnectWhatsApp(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE whatsapp_integrations SET is_active = false WHERE company_id = $1
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "✅ WhatsApp Business отключён"})
}

// SendWhatsAppMessage - отправка сообщения
func SendWhatsAppMessage(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        To          string `json:"to" binding:"required"`
        Message     string `json:"message"`
        Type        string `json:"type"`
        MediaURL    string `json:"media_url"`
        Caption     string `json:"caption"`
        TemplateID  string `json:"template_id"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем настройки интеграции
    var phoneNumberID, accessToken string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT phone_number_id, access_token FROM whatsapp_integrations
        WHERE company_id = $1 AND is_active = true
    `, companyID).Scan(&phoneNumberID, &accessToken)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "WhatsApp not connected"})
        return
    }

    // Отправка через WhatsApp Cloud API
    url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)

    var payload map[string]interface{}
    
    switch req.Type {
    case "image":
        payload = map[string]interface{}{
            "messaging_product": "whatsapp",
            "to": req.To,
            "type": "image",
            "image": map[string]string{
                "link": req.MediaURL,
                "caption": req.Caption,
            },
        }
    case "document":
        payload = map[string]interface{}{
            "messaging_product": "whatsapp",
            "to": req.To,
            "type": "document",
            "document": map[string]string{
                "link": req.MediaURL,
                "filename": req.Caption,
            },
        }
    case "template":
        payload = map[string]interface{}{
            "messaging_product": "whatsapp",
            "to": req.To,
            "type": "template",
            "template": map[string]interface{}{
                "name": req.TemplateID,
                "language": map[string]string{"code": "ru"},
            },
        }
    default:
        payload = map[string]interface{}{
            "messaging_product": "whatsapp",
            "to": req.To,
            "type": "text",
            "text": map[string]string{"body": req.Message},
        }
    }

    jsonPayload, _ := json.Marshal(payload)
    
    httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
    httpReq.Header.Set("Authorization", "Bearer "+accessToken)
    httpReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(httpReq)
    if err != nil {
        log.Printf("❌ Ошибка отправки: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    if resp.StatusCode != http.StatusOK {
        c.JSON(resp.StatusCode, gin.H{"error": string(body)})
        return
    }

    // Сохраняем сообщение в историю
    messageID := uuid.New().String()
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_messages (id, company_id, from_number, to_number, message, type, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, 'sent', NOW())
    `, messageID, companyID, phoneNumberID, req.To, req.Message, req.Type)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Сообщение отправлено!",
    })
}

// GetWhatsAppMessages - получение истории сообщений
func GetWhatsAppMessages(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    limit := c.DefaultQuery("limit", "50")
    offset := c.DefaultQuery("offset", "0")
    contact := c.Query("contact")

    var query string
    var args []interface{}

    if contact != "" {
        query = `
            SELECT id, from_number, to_number, message, type, status, created_at 
            FROM whatsapp_messages 
            WHERE company_id = $1 AND (from_number = $2 OR to_number = $2)
            ORDER BY created_at DESC 
            LIMIT $3 OFFSET $4
        `
        args = []interface{}{companyID, contact, limit, offset}
    } else {
        query = `
            SELECT id, from_number, to_number, message, type, status, created_at 
            FROM whatsapp_messages 
            WHERE company_id = $1 
            ORDER BY created_at DESC 
            LIMIT $2 OFFSET $3
        `
        args = []interface{}{companyID, limit, offset}
    }

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"messages": []WhatsAppMessage{}})
        return
    }
    defer rows.Close()

    var messages []WhatsAppMessage
    for rows.Next() {
        var msg WhatsAppMessage
        rows.Scan(&msg.ID, &msg.From, &msg.To, &msg.Body, &msg.Type, &msg.Status, &msg.CreatedAt)
        messages = append(messages, msg)
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "messages": messages,
        "count": len(messages),
    })
}

// GetWhatsAppTemplates - получение шаблонов сообщений
func GetWhatsAppTemplates(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, content, category, created_at 
        FROM whatsapp_templates 
        WHERE company_id = $1 
        ORDER BY created_at DESC
    `, companyID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"templates": []WhatsAppTemplate{}})
        return
    }
    defer rows.Close()

    var templates []WhatsAppTemplate
    for rows.Next() {
        var t WhatsAppTemplate
        rows.Scan(&t.ID, &t.Name, &t.Body, &t.Category, &t.CreatedAt)
        templates = append(templates, t)
    }

    c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// CreateWhatsAppTemplate - создание шаблона
func CreateWhatsAppTemplate(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name     string `json:"name" binding:"required"`
        Body     string `json:"content" binding:"required"`
        Category string `json:"category"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    templateID := uuid.New().String()
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_templates (id, company_id, name, content, category, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
    `, templateID, companyID, req.Name, req.Body, req.Category)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Шаблон создан!",
        "template_id": templateID,
    })
}

// UpdateWhatsAppTemplate - обновление шаблона
func UpdateWhatsAppTemplate(c *gin.Context) {
    templateID := c.Param("id")
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name     string `json:"name"`
        Body     string `json:"content"`
        Category string `json:"category"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    query := `UPDATE whatsapp_templates SET updated_at = NOW()`
    args := []interface{}{}
    i := 1

    if req.Name != "" {
        query += fmt.Sprintf(", name = $%d", i+1)
        args = append(args, req.Name)
        i++
    }
    if req.Body != "" {
        query += fmt.Sprintf(", content = $%d", i+1)
        args = append(args, req.Body)
        i++
    }
    if req.Category != "" {
        query += fmt.Sprintf(", category = $%d", i+1)
        args = append(args, req.Category)
        i++
    }

    query += fmt.Sprintf(" WHERE id = $%d AND company_id = $%d", i+1, i+2)
    args = append(args, templateID, companyID)

    _, err := database.Pool.Exec(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Шаблон обновлён!",
    })
}

// DeleteWhatsAppTemplate - удаление шаблона
func DeleteWhatsAppTemplate(c *gin.Context) {
    templateID := c.Param("id")
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM whatsapp_templates WHERE id = $1 AND company_id = $2
    `, templateID, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Шаблон удалён",
    })
}

// CreateWhatsAppBroadcast - создание рассылки
func CreateWhatsAppBroadcast(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name       string   `json:"name" binding:"required"`
        Message    string   `json:"message"`
        TemplateID string   `json:"template_id"`
        Recipients []string `json:"recipients" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    recipientsJSON, _ := json.Marshal(req.Recipients)
    broadcastID := uuid.New().String()

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_broadcasts (id, company_id, name, message, template_id, recipients, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, 'pending', NOW())
    `, broadcastID, companyID, req.Name, req.Message, req.TemplateID, recipientsJSON)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Рассылка создана!",
        "broadcast_id": broadcastID,
    })
}

// GetWhatsAppBroadcasts - список рассылок
func GetWhatsAppBroadcasts(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, message, template_id, status, sent_count, failed_count, created_at 
        FROM whatsapp_broadcasts 
        WHERE company_id = $1 
        ORDER BY created_at DESC
    `, companyID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"broadcasts": []WhatsAppBroadcast{}})
        return
    }
    defer rows.Close()

    var broadcasts []WhatsAppBroadcast
    for rows.Next() {
        var b WhatsAppBroadcast
        var templateID *string
        rows.Scan(&b.ID, &b.Name, &b.Message, &templateID, &b.Status, &b.SentCount, &b.FailedCount, &b.CreatedAt)
        if templateID != nil {
            b.TemplateID = *templateID
        }
        broadcasts = append(broadcasts, b)
    }

    c.JSON(http.StatusOK, gin.H{"broadcasts": broadcasts})
}

// SendWhatsAppBroadcast - отправка рассылки
func SendWhatsAppBroadcast(c *gin.Context) {
    broadcastID := c.Param("id")
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var name, message string
    var templateID *string
    var recipientsJSON []byte
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT name, message, template_id, recipients FROM whatsapp_broadcasts 
        WHERE id = $1 AND company_id = $2
    `, broadcastID, companyID).Scan(&name, &message, &templateID, &recipientsJSON)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Broadcast not found"})
        return
    }

    var recipients []string
    json.Unmarshal(recipientsJSON, &recipients)

    var phoneNumberID, accessToken string
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT phone_number_id, access_token FROM whatsapp_integrations
        WHERE company_id = $1 AND is_active = true
    `, companyID).Scan(&phoneNumberID, &accessToken)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "WhatsApp not connected"})
        return
    }

    sentCount := 0
    failedCount := 0

    for _, to := range recipients {
        url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)
        
        var payload map[string]interface{}
        
        if templateID != nil && *templateID != "" {
            payload = map[string]interface{}{
                "messaging_product": "whatsapp",
                "to": to,
                "type": "template",
                "template": map[string]interface{}{
                    "name": *templateID,
                    "language": map[string]string{"code": "ru"},
                },
            }
        } else {
            payload = map[string]interface{}{
                "messaging_product": "whatsapp",
                "to": to,
                "type": "text",
                "text": map[string]string{"body": message},
            }
        }
        
        jsonPayload, _ := json.Marshal(payload)
        httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
        httpReq.Header.Set("Authorization", "Bearer "+accessToken)
        httpReq.Header.Set("Content-Type", "application/json")

        client := &http.Client{Timeout: 30 * time.Second}
        resp, err := client.Do(httpReq)
        
        if err != nil || resp.StatusCode != http.StatusOK {
            failedCount++
        } else {
            sentCount++
        }
        
        if resp != nil {
            resp.Body.Close()
        }
        
        time.Sleep(500 * time.Millisecond)
    }

    status := "completed"
    if failedCount > 0 && sentCount == 0 {
        status = "failed"
    } else if failedCount > 0 {
        status = "partial"
    }

    database.Pool.Exec(c.Request.Context(), `
        UPDATE whatsapp_broadcasts 
        SET status = $1, sent_count = $2, failed_count = $3, sent_at = NOW()
        WHERE id = $4
    `, status, sentCount, failedCount, broadcastID)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": fmt.Sprintf("✅ Рассылка '%s' отправлена!", name),
        "sent": sentCount,
        "failed": failedCount,
        "total": len(recipients),
    })
}

// DeleteWhatsAppBroadcast - удаление рассылки
func DeleteWhatsAppBroadcast(c *gin.Context) {
    broadcastID := c.Param("id")
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM whatsapp_broadcasts WHERE id = $1 AND company_id = $2
    `, broadcastID, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "✅ Рассылка удалена",
    })
}

// GetWhatsAppContacts - список контактов
func GetWhatsAppContacts(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT DISTINCT 
            CASE 
                WHEN from_number != $1 THEN from_number 
                ELSE to_number 
            END as phone_number,
            MAX(created_at) as last_active
        FROM whatsapp_messages 
        WHERE company_id = $2
        GROUP BY phone_number
        ORDER BY last_active DESC
        LIMIT 100
    `, "placeholder", companyID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"contacts": []WhatsAppContact{}})
        return
    }
    defer rows.Close()

    var contacts []WhatsAppContact
    for rows.Next() {
        var c WhatsAppContact
        rows.Scan(&c.PhoneNumber, &c.LastActive)
        contacts = append(contacts, c)
    }

    c.JSON(http.StatusOK, gin.H{"contacts": contacts})
}

// GetWhatsAppStats - статистика
func GetWhatsAppStats(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var stats WhatsAppStats
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM whatsapp_messages WHERE company_id = $1
    `, companyID).Scan(&stats.TotalMessages)
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM whatsapp_messages WHERE company_id = $1 AND status = 'sent'
    `, companyID).Scan(&stats.SentMessages)
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM whatsapp_messages WHERE company_id = $1 AND status = 'received'
    `, companyID).Scan(&stats.ReceivedMessages)
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM whatsapp_templates WHERE company_id = $1
    `, companyID).Scan(&stats.TotalTemplates)
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM whatsapp_broadcasts WHERE company_id = $1
    `, companyID).Scan(&stats.TotalBroadcasts)
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(DISTINCT CASE WHEN from_number != $1 THEN from_number ELSE to_number END)
        FROM whatsapp_messages WHERE company_id = $2
    `, "placeholder", companyID).Scan(&stats.TotalContacts)

    c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// GetWhatsAppMessageStats - статистика по сообщениям (график)
func GetWhatsAppMessageStats(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    days := c.DefaultQuery("days", "7")
    daysInt, _ := strconv.Atoi(days)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT DATE(created_at) as date, COUNT(*) as count, 
               SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END) as sent,
               SUM(CASE WHEN status = 'received' THEN 1 ELSE 0 END) as received
        FROM whatsapp_messages 
        WHERE company_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
        GROUP BY DATE(created_at)
        ORDER BY date ASC
    `, companyID, daysInt)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"stats": []interface{}{}})
        return
    }
    defer rows.Close()
    
    var chartData []gin.H
    for rows.Next() {
        var date time.Time
        var count, sent, received int
        rows.Scan(&date, &count, &sent, &received)
        chartData = append(chartData, gin.H{
            "date": date.Format("2006-01-02"),
            "total": count,
            "sent": sent,
            "received": received,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"stats": chartData})
}

// WhatsAppWebhook - вебхук для входящих сообщений
func WhatsAppWebhook(c *gin.Context) {
    if c.Request.Method == "GET" {
        mode := c.Query("hub.mode")
        token := c.Query("hub.verify_token")
        challenge := c.Query("hub.challenge")
        
        var verifyToken string
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT webhook_verify_token FROM whatsapp_integrations WHERE is_active = true LIMIT 1
        `).Scan(&verifyToken)
        
        if mode == "subscribe" && token == verifyToken {
            c.String(http.StatusOK, challenge)
            return
        }
        c.String(http.StatusForbidden, "Forbidden")
        return
    }

    var body map[string]interface{}
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    log.Printf("📨 Входящее WhatsApp сообщение: %v", body)
    
    // Парсим и сохраняем входящее сообщение
    if entries, ok := body["entry"].([]interface{}); ok {
        for _, entry := range entries {
            if changes, ok := entry.(map[string]interface{})["changes"].([]interface{}); ok {
                for _, change := range changes {
                    if value, ok := change.(map[string]interface{})["value"].(map[string]interface{}); ok {
                        if messages, ok := value["messages"].([]interface{}); ok {
                            for _, msg := range messages {
                                if msgMap, ok := msg.(map[string]interface{}); ok {
                                    from := msgMap["from"].(string)
                                    text := ""
                                    if textObj, ok := msgMap["text"].(map[string]interface{}); ok {
                                        text = textObj["body"].(string)
                                    }
                                    
                                    companyID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
                                    database.Pool.Exec(c.Request.Context(), `
                                        INSERT INTO whatsapp_messages (id, company_id, from_number, to_number, message, type, status, created_at)
                                        VALUES ($1, $2, $3, $4, $5, 'received', 'received', NOW())
                                    `, uuid.New().String(), companyID, from, "", text)
                                    
                                    log.Printf("📨 Сообщение от %s: %s", from, text)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}