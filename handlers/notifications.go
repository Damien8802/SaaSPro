package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/smtp"
    "os"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/config"
    "subscription-system/database"
)

// Notification types
const (
    NotifLoginNewDevice   = "new_device_login"
    Notif2FAEnabled       = "2fa_enabled"
    Notif2FADisabled      = "2fa_disabled"
    NotifPasswordChanged  = "password_changed"
    NotifDeviceTrusted    = "device_trusted"
    NotifDeviceRevoked    = "device_revoked"
    NotifSuspiciousLogin  = "suspicious_login"
)

// EmailConfig структура настроек SMTP
type EmailConfig struct {
    Host     string
    Port     int
    Username string
    Password string
    From     string
    FromName string
    TLS      bool
}

var emailConfig *EmailConfig

// InitEmailService - инициализация email сервиса
func InitEmailService(cfg *config.Config) {
    emailConfig = &EmailConfig{
        Host:     cfg.SMTPHost,
        Port:     cfg.SMTPPort,
        Username: cfg.SMTPUser,
        Password: cfg.SMTPPassword,
        From:     cfg.SMTPFrom,
        FromName: cfg.SMTPFromName,
        TLS:      cfg.SMTPTLS,
    }
    
    if emailConfig.Host != "" {
        log.Printf("📧 Email сервис инициализирован: %s:%d", emailConfig.Host, emailConfig.Port)
    } else {
        log.Println("⚠️ Email сервис не настроен (SMTP_HOST не задан)")
    }
}

// SendEmailNotification - отправка email уведомления
func SendEmailNotification(recipient, subject, body string) error {
    if emailConfig == nil || emailConfig.Host == "" {
        return fmt.Errorf("email service not configured")
    }
    
    auth := smtp.PlainAuth("", emailConfig.Username, emailConfig.Password, emailConfig.Host)
    
    to := []string{recipient}
    msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", recipient, subject, body))
    
    addr := fmt.Sprintf("%s:%d", emailConfig.Host, emailConfig.Port)
    
    if emailConfig.TLS {
        return smtp.SendMail(addr, auth, emailConfig.From, to, msg)
    }
    
    return smtp.SendMail(addr, auth, emailConfig.From, to, msg)
}

// QueueEmail - добавить email в очередь
func QueueEmail(userID uuid.UUID, recipient, subject, body string) {
    _, err := database.Pool.Exec(context.Background(), `
        INSERT INTO notification_queue (user_id, type, channel, recipient, subject, content, status, created_at)
        VALUES ($1, 'email', 'email', $2, $3, $4, 'pending', NOW())
    `, userID, recipient, subject, body)
    
    if err != nil {
        log.Printf("❌ Ошибка добавления email в очередь: %v", err)
    }
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

// GetLocationByIP определяет местоположение по IP
func GetLocationByIP(ip string) string {
    if ip == "::1" || ip == "127.0.0.1" {
        return "Локальный доступ"
    }

    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get("http://ip-api.com/json/" + ip + "?lang=ru&fields=status,country,city,isp,query")
    if err != nil {
        log.Printf("⚠️ Ошибка определения местоположения для IP %s: %v", ip, err)
        return "Неизвестно"
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    var result struct {
        Status  string `json:"status"`
        Country string `json:"country"`
        City    string `json:"city"`
        ISP     string `json:"isp"`
    }
    
    if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" {
        return "Неизвестно"
    }
    
    if result.City != "" && result.Country != "" {
        return fmt.Sprintf("%s, %s (%s)", result.City, result.Country, result.ISP)
    }
    if result.Country != "" {
        return result.Country
    }
    return "Неизвестно"
}

// SendTelegramMessage отправляет сообщение в Telegram
func SendTelegramMessage(chatID string, message string) error {
    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    if botToken == "" {
        return fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
    }
    
    var chatIDInt int64
    fmt.Sscanf(chatID, "%d", &chatIDInt)
    
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
    
    payload := map[string]interface{}{
        "chat_id":    chatIDInt,
        "text":       message,
        "parse_mode": "HTML",
    }
    
    jsonData, _ := json.Marshal(payload)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// SendTelegramNotification отправляет уведомление пользователю в Telegram
func SendTelegramNotification(userID uuid.UUID, message string) error {
    var telegramID int64
    err := database.Pool.QueryRow(context.Background(),
        "SELECT telegram_id FROM users WHERE id = $1", userID).Scan(&telegramID)
    if err != nil || telegramID == 0 {
        return fmt.Errorf("telegram ID not found for user %s", userID)
    }

    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    if botToken == "" {
        return fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
    }

    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
    
    payload := map[string]interface{}{
        "chat_id":    telegramID,
        "text":       message,
        "parse_mode": "HTML",
    }
    
    jsonData, _ := json.Marshal(payload)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// ========== ОСНОВНЫЕ ФУНКЦИИ УВЕДОМЛЕНИЙ ==========

// GetNotifications возвращает список уведомлений пользователя
func GetNotifications(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, type, title, message, link, is_read, created_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var notifications []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var notifType, title, message, link string
        var isRead bool
        var createdAt time.Time
        
        rows.Scan(&id, &notifType, &title, &message, &link, &isRead, &createdAt)
        
        notifications = append(notifications, map[string]interface{}{
            "id":         id,
            "type":       notifType,
            "title":      title,
            "message":    message,
            "link":       link,
            "is_read":    isRead,
            "created_at": createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// MarkNotificationRead отмечает уведомление как прочитанное
func MarkNotificationRead(c *gin.Context) {
    userID := getUserID(c)
    notificationID := c.Param("id")
    
    if notificationID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "notification id required"})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE notifications SET is_read = true
        WHERE id = $1 AND user_id = $2
    `, notificationID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
func GetUnreadCount(c *gin.Context) {
    userID := getUserID(c)
    
    var count int
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM notifications
        WHERE user_id = $1 AND is_read = false
    `, userID).Scan(&count)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

// GetUserNotificationSettings - получить настройки уведомлений
func GetUserNotificationSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var settings struct {
        EmailEnabled         bool   `json:"email_enabled"`
        TelegramEnabled      bool   `json:"telegram_enabled"`
        PushEnabled          bool   `json:"push_enabled"`
        Email                string `json:"email"`
        TelegramChatID       string `json:"telegram_chat_id"`
        NotifyOnOrderCreated bool   `json:"notify_on_order_created"`
        NotifyOnOrderStatus  bool   `json:"notify_on_order_status"`
        NotifyOnLowStock     bool   `json:"notify_on_low_stock"`
        NotifyOnPayment      bool   `json:"notify_on_payment"`
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COALESCE(ns.email_enabled, true) as email_enabled,
            COALESCE(ns.telegram_enabled, false) as telegram_enabled,
            COALESCE(ns.push_enabled, false) as push_enabled,
            COALESCE(u.email, '') as email,
            COALESCE(ns.telegram_chat_id, '') as telegram_chat_id,
            COALESCE(ns.notify_on_order_created, true) as notify_on_order_created,
            COALESCE(ns.notify_on_order_status, true) as notify_on_order_status,
            COALESCE(ns.notify_on_low_stock, true) as notify_on_low_stock,
            COALESCE(ns.notify_on_payment, true) as notify_on_payment
        FROM users u
        LEFT JOIN user_notification_settings ns ON u.id = ns.user_id
        WHERE u.id = $1
    `, userID).Scan(
        &settings.EmailEnabled, &settings.TelegramEnabled, &settings.PushEnabled,
        &settings.Email, &settings.TelegramChatID,
        &settings.NotifyOnOrderCreated, &settings.NotifyOnOrderStatus,
        &settings.NotifyOnLowStock, &settings.NotifyOnPayment,
    )
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "settings": settings,
    })
}

// UpdateUserNotificationSettings - обновить настройки уведомлений
func UpdateUserNotificationSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        EmailEnabled         bool   `json:"email_enabled"`
        TelegramEnabled      bool   `json:"telegram_enabled"`
        PushEnabled          bool   `json:"push_enabled"`
        TelegramChatID       string `json:"telegram_chat_id"`
        NotifyOnOrderCreated bool   `json:"notify_on_order_created"`
        NotifyOnOrderStatus  bool   `json:"notify_on_order_status"`
        NotifyOnLowStock     bool   `json:"notify_on_low_stock"`
        NotifyOnPayment      bool   `json:"notify_on_payment"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO user_notification_settings (
            user_id, email_enabled, telegram_enabled, push_enabled,
            telegram_chat_id, notify_on_order_created, notify_on_order_status,
            notify_on_low_stock, notify_on_payment, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        ON CONFLICT (user_id) DO UPDATE SET
            email_enabled = EXCLUDED.email_enabled,
            telegram_enabled = EXCLUDED.telegram_enabled,
            push_enabled = EXCLUDED.push_enabled,
            telegram_chat_id = EXCLUDED.telegram_chat_id,
            notify_on_order_created = EXCLUDED.notify_on_order_created,
            notify_on_order_status = EXCLUDED.notify_on_order_status,
            notify_on_low_stock = EXCLUDED.notify_on_low_stock,
            notify_on_payment = EXCLUDED.notify_on_payment,
            updated_at = NOW()
    `, userID, req.EmailEnabled, req.TelegramEnabled, req.PushEnabled,
        req.TelegramChatID, req.NotifyOnOrderCreated, req.NotifyOnOrderStatus,
        req.NotifyOnLowStock, req.NotifyOnPayment)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки сохранены",
    })
}

// SendTestNotification - отправка тестового уведомления
func SendTestNotification(c *gin.Context) {
    userID := getUserID(c)
    
    var email string
    var telegramChatID string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COALESCE(u.email, ''),
            COALESCE(ns.telegram_chat_id, '')
        FROM users u
        LEFT JOIN user_notification_settings ns ON u.id = ns.user_id
        WHERE u.id = $1
    `, userID).Scan(&email, &telegramChatID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось получить настройки",
            "details": err.Error(),
        })
        return
    }
    
    if telegramChatID != "" && telegramChatID != "0" {
        go SendTelegramMessage(telegramChatID, "🧪 Тестовое уведомление из SaaSPro")
    }
    
    if email != "" {
        go func() {
            subject := "🧪 Тестовое уведомление от SaaSPro"
            body := "<h2>Тест!</h2><p>Это тестовое уведомление из SaaSPro.</p>"
            if err := SendEmailNotification(email, subject, body); err != nil {
                log.Printf("❌ Ошибка отправки тестового email: %v", err)
            } else {
                log.Printf("✅ Тестовое email отправлено на %s", email)
            }
        }()
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Тестовое уведомление отправлено",
    })
}

// LogAndNotify логирует событие и отправляет уведомления
func LogAndNotify(c *gin.Context, userID uuid.UUID, notifType string, details map[string]interface{}) {
    if notifType == NotifLoginNewDevice || notifType == NotifSuspiciousLogin {
        if ip, ok := details["ip"].(string); ok && ip != "" {
            location := GetLocationByIP(ip)
            details["location"] = location
        }
    }

    _, err := database.Pool.Exec(context.Background(), `
        INSERT INTO notification_log (user_id, type, details, created_at) 
        VALUES ($1, $2, $3, $4)`,
        userID, notifType, details, time.Now())
    
    if err != nil {
        log.Printf("❌ Ошибка логирования уведомления: %v", err)
    }
    
    message := formatNotificationMessage(notifType, details)
    
    go func() {
        var telegramID int64
        err := database.Pool.QueryRow(context.Background(),
            "SELECT telegram_id FROM users WHERE id = $1", userID).Scan(&telegramID)
        if err == nil && telegramID != 0 {
            SendTelegramMessage(fmt.Sprintf("%d", telegramID), message)
        }
    }()
}

func formatNotificationMessage(notifType string, details map[string]interface{}) string {
    switch notifType {
    case NotifLoginNewDevice:
        return fmt.Sprintf(`🚨 <b>НОВЫЙ ВХОД В АККАУНТ</b>

📍 <b>IP:</b> <code>%v</code>
🌍 <b>Местоположение:</b> %v
💻 <b>Устройство:</b> %v
⏰ <b>Время:</b> %v

⚠️ <b>Если это были не вы:</b>
1️⃣ Немедленно смените пароль
2️⃣ Проверьте доверенные устройства
3️⃣ Включите 2FA

✅ <b>Если это вы</b> — можете добавить устройство в доверенные в настройках безопасности.`,
            details["ip"], details["location"], details["device"], details["time"])

    case Notif2FAEnabled:
        return "🔒 <b>✅ 2FA ВКЛЮЧЕНА</b>\n\nДвухфакторная аутентификация успешно активирована для вашего аккаунта. Ваш аккаунт теперь под дополнительной защитой!"

    case Notif2FADisabled:
        return "🔓 <b>⚠️ 2FA ОТКЛЮЧЕНА</b>\n\nДвухфакторная аутентификация была отключена. Если это были не вы, срочно примите меры!"

    case NotifPasswordChanged:
        return "🔑 <b>✅ ПАРОЛЬ ИЗМЕНЁН</b>\n\nПароль от вашего аккаунта был успешно изменён."

    case NotifDeviceTrusted:
        return fmt.Sprintf(`📱 <b>✅ НОВОЕ ДОВЕРЕННОЕ УСТРОЙСТВО</b>
        
<b>Устройство:</b> %v
<b>IP:</b> <code>%v</code>
<b>Срок действия:</b> 30 дней

Теперь вход с этого устройства не требует подтверждения.`,
            details["device"], details["ip"])

    case NotifDeviceRevoked:
        return fmt.Sprintf(`🚫 <b>🔐 ДОСТУП УСТРОЙСТВА ОТОЗВАН</b>
        
<b>Устройство:</b> %v больше не имеет доступа к вашему аккаунту.`,
            details["device"])

    case NotifSuspiciousLogin:
        return fmt.Sprintf(`🚨 <b>⚠️ ПОДОЗРИТЕЛЬНАЯ АКТИВНОСТЬ</b>
        
Обнаружена подозрительная попытка входа:
📍 <b>IP:</b> <code>%v</code>
🌍 <b>Местоположение:</b> %v
💻 <b>Устройство:</b> %v

<b>Рекомендуем:</b>
• Немедленно сменить пароль
• Проверить список доверенных устройств
• Включить 2FA, если ещё не сделано`,
            details["ip"], details["location"], details["device"])

    default:
        return "⚠️ Уведомление от системы безопасности"
    }
}

// NotifyPageHandler - страница уведомлений
func NotifyPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "notify.html", gin.H{
        "title": "Уведомления | SaaSPro",
    })
}

// NotifyHandler - API для отправки уведомлений
func NotifyHandler(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Title   string `json:"title" binding:"required"`
        Message string `json:"message" binding:"required"`
        Type    string `json:"type"`
        Link    string `json:"link"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.Type == "" {
        req.Type = "info"
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO notifications (user_id, type, title, message, link, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
    `, userID, req.Type, req.Title, req.Message, req.Link)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Уведомление отправлено",
    })
}