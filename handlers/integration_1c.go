package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// ProductXML структура для экспорта товаров в XML (формат 1С)
type ProductXML struct {
    XMLName   xml.Name `xml:"Товар"`
    Code      string   `xml:"Код"`
    Name      string   `xml:"Наименование"`
    SKU       string   `xml:"Артикул"`
    Price     float64  `xml:"Цена"`
    Quantity  int      `xml:"Количество"`
    Unit      string   `xml:"ЕдиницаИзмерения"`
}

// ProductsXML обертка для списка товаров
type ProductsXML struct {
    XMLName  xml.Name     `xml:"Товары"`
    Products []ProductXML `xml:"Товар"`
}

// OrderXML структура для экспорта заказов
type OrderXML struct {
    XMLName      xml.Name   `xml:"Заказ"`
    Number       string     `xml:"Номер"`
    Date         string     `xml:"Дата"`
    CustomerName string     `xml:"Покупатель>Наименование"`
    TotalAmount  float64    `xml:"Сумма"`
    Items        []ItemXML  `xml:"Товары>Товар"`
}

type ItemXML struct {
    Code     string  `xml:"Код"`
    Name     string  `xml:"Наименование"`
    Quantity int     `xml:"Количество"`
    Price    float64 `xml:"Цена"`
    Amount   float64 `xml:"Сумма"`
}

// ==================== ЭКСПОРТ В 1С ====================

// ExportProductsTo1C - экспорт товаров в XML формате 1С
func ExportProductsTo1C(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, COALESCE(sku, ''), price, quantity, COALESCE(unit, 'шт')
        FROM products
        WHERE user_id = $1 AND active = true
    `, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var products []ProductXML
    for rows.Next() {
        var p ProductXML
        var id uuid.UUID
        rows.Scan(&id, &p.Name, &p.SKU, &p.Price, &p.Quantity, &p.Unit)
        p.Code = id.String()
        products = append(products, p)
    }
    
    xmlData := ProductsXML{Products: products}
    
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'export', 'products', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(products)).Scan(&logID)
    
    filename := fmt.Sprintf("export_products_%d.xml", time.Now().Unix())
    filepath := fmt.Sprintf("./exports/%s", filename)
    
    os.MkdirAll("./exports", 0755)
    
    file, err := os.Create(filepath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать файл"})
        return
    }
    defer file.Close()
    
    encoder := xml.NewEncoder(file)
    encoder.Indent("", "  ")
    if err := encoder.Encode(xmlData); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации XML"})
        return
    }
    
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = 'completed', completed_at = NOW(), file_path = $1
        WHERE id = $2
    `, filepath, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "message":   "Экспорт выполнен",
        "file":      filename,
        "count":     len(products),
        "log_id":    logID,
    })
}

// ExportOrdersTo1C - экспорт заказов в XML
func ExportOrdersTo1C(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT o.id, o.order_number, o.created_at, o.customer_name, o.total_amount
        FROM orders o
        WHERE o.user_id = $1
        ORDER BY o.created_at DESC
        LIMIT 100
    `, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var orders []OrderXML
    for rows.Next() {
        var o OrderXML
        var id uuid.UUID
        var createdAt time.Time
        rows.Scan(&id, &o.Number, &createdAt, &o.CustomerName, &o.TotalAmount)
        o.Date = createdAt.Format("2006-01-02")
        
        itemsRows, err := database.Pool.Query(c.Request.Context(), `
            SELECT product_name, sku, quantity, price, total
            FROM order_items
            WHERE order_id = $1
        `, id)
        if err == nil {
            for itemsRows.Next() {
                var item ItemXML
                itemsRows.Scan(&item.Name, &item.Code, &item.Quantity, &item.Price, &item.Amount)
                o.Items = append(o.Items, item)
            }
            itemsRows.Close()
        }
        
        orders = append(orders, o)
    }
    
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'export', 'orders', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(orders)).Scan(&logID)
    
    filename := fmt.Sprintf("export_orders_%d.json", time.Now().Unix())
    filepath := fmt.Sprintf("./exports/%s", filename)
    
    os.MkdirAll("./exports", 0755)
    
    file, err := os.Create(filepath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать файл"})
        return
    }
    defer file.Close()
    
    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(orders); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации JSON"})
        return
    }
    
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = 'completed', completed_at = NOW(), file_path = $1
        WHERE id = $2
    `, filepath, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Экспорт заказов выполнен",
        "file":    filename,
        "count":   len(orders),
    })
}

// ==================== ИМПОРТ ИЗ 1С ====================

// ImportProductsFrom1C - импорт товаров из 1С XML
func ImportProductsFrom1C(c *gin.Context) {
    userID := getUserID(c)
    
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не выбран"})
        return
    }
    
    tempPath := fmt.Sprintf("./uploads/%s", file.Filename)
    os.MkdirAll("./uploads", 0755)
    
    if err := c.SaveUploadedFile(file, tempPath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить файл"})
        return
    }
    defer os.Remove(tempPath)
    
    f, err := os.Open(tempPath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось открыть файл"})
        return
    }
    defer f.Close()
    
    decoder := xml.NewDecoder(f)
    var products ProductsXML
    
    if err := decoder.Decode(&products); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка парсинга XML: " + err.Error()})
        return
    }
    
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'import', 'products', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(products.Products)).Scan(&logID)
    
    var imported int
    var errors []string
    
    for _, p := range products.Products {
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO products (user_id, name, sku, price, quantity, unit, active, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
            ON CONFLICT (user_id, sku) DO UPDATE SET
                name = EXCLUDED.name,
                price = EXCLUDED.price,
                quantity = EXCLUDED.quantity,
                updated_at = NOW()
        `, userID, p.Name, p.SKU, p.Price, p.Quantity, p.Unit)
        
        if err != nil {
            errors = append(errors, fmt.Sprintf("%s: %v", p.Name, err))
        } else {
            imported++
        }
    }
    
    status := "completed"
    errorMsg := ""
    if len(errors) > 0 {
        status = "partial"
        errorMsg = fmt.Sprintf("Импортировано %d из %d, ошибок: %d", imported, len(products.Products), len(errors))
    }
    
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = $1, completed_at = NOW(), error_message = $2
        WHERE id = $3
    `, status, errorMsg, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "imported":  imported,
        "total":     len(products.Products),
        "errors":    errors,
        "message":   fmt.Sprintf("Импортировано %d товаров", imported),
        "log_id":    logID,
    })
}

// ==================== ЖУРНАЛЫ СИНХРОНИЗАЦИИ ====================

// GetSyncLogs - получить логи синхронизации
func GetSyncLogs(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, direction, entity_type, record_count, status, error_message, file_path, started_at, completed_at
        FROM sync_logs
        WHERE user_id = $1
        ORDER BY started_at DESC
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
        var direction, entityType, status, errorMsg, filePath string
        var recordCount int
        var startedAt, completedAt time.Time
        var completedAtPtr *time.Time
        
        rows.Scan(&id, &direction, &entityType, &recordCount, &status, &errorMsg, &filePath, &startedAt, &completedAt)
        
        if !completedAt.IsZero() {
            completedAtPtr = &completedAt
        }
        
        logs = append(logs, map[string]interface{}{
            "id":            id,
            "direction":     direction,
            "entity_type":   entityType,
            "record_count":  recordCount,
            "status":        status,
            "error_message": errorMsg,
            "file_path":     filePath,
            "started_at":    startedAt,
            "completed_at":  completedAtPtr,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "logs":    logs,
    })
}

// GetSyncSettings - получить настройки интеграции
func GetSyncSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var settingsJSON []byte
    var lastSync time.Time
    var syncStatus string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT settings, last_sync, sync_status
        FROM integration_settings
        WHERE user_id = $1 AND integration_type = '1c'
    `, userID).Scan(&settingsJSON, &lastSync, &syncStatus)
    
    if err != nil {
        defaultSettings := map[string]interface{}{
            "auto_sync":        false,
            "sync_interval":    3600,
            "export_products":  true,
            "export_orders":    true,
            "import_products":  false,
            "last_sync_status": "never",
        }
        c.JSON(http.StatusOK, gin.H{
            "success":   true,
            "settings":  defaultSettings,
            "last_sync": nil,
            "status":    "idle",
        })
        return
    }
    
    var settings map[string]interface{}
    json.Unmarshal(settingsJSON, &settings)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "settings":  settings,
        "last_sync": lastSync,
        "status":    syncStatus,
    })
}

// UpdateSyncSettings - обновить настройки интеграции
func UpdateSyncSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Settings map[string]interface{} `json:"settings" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    settingsJSON, _ := json.Marshal(req.Settings)
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO integration_settings (user_id, integration_type, settings, updated_at)
        VALUES ($1, '1c', $2, NOW())
        ON CONFLICT (user_id, integration_type) DO UPDATE SET
            settings = EXCLUDED.settings,
            updated_at = NOW()
    `, userID, settingsJSON)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить настройки"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки сохранены",
    })
}

// ========== АВТОМАТИЧЕСКАЯ СИНХРОНИЗАЦИЯ ==========

// SyncScheduler - планировщик синхронизации
type SyncScheduler struct {
    ticker *time.Ticker
    stop   chan bool
}

var syncScheduler *SyncScheduler

// StartSyncScheduler - запуск планировщика
func StartSyncScheduler() {
    if syncScheduler != nil {
        return
    }
    
    syncScheduler = &SyncScheduler{
        stop: make(chan bool),
    }
    
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                checkAndRunSync()
            case <-syncScheduler.stop:
                return
            }
        }
    }()
    
    log.Println("🤖 Планировщик синхронизации с 1С запущен")
}

// StopSyncScheduler - остановка планировщика
func StopSyncScheduler() {
    if syncScheduler != nil {
        syncScheduler.stop <- true
        log.Println("🛑 Планировщик синхронизации с 1С остановлен")
    }
}

// checkAndRunSync - проверка настроек и запуск синхронизации
func checkAndRunSync() {
    ctx := context.Background()
    
    rows, err := database.Pool.Query(ctx, `
        SELECT user_id, settings
        FROM integration_settings
        WHERE integration_type = '1c' AND settings->>'auto_sync' = 'true'
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка проверки настроек синхронизации: %v", err)
        return
    }
    defer rows.Close()
    
    for rows.Next() {
        var userID uuid.UUID
        var settingsJSON []byte
        rows.Scan(&userID, &settingsJSON)
        
        var settings map[string]interface{}
        json.Unmarshal(settingsJSON, &settings)
        
        interval := 3600
        if val, ok := settings["sync_interval"]; ok {
            if v, ok := val.(float64); ok {
                interval = int(v)
            }
        }
        
        var lastSync time.Time
        database.Pool.QueryRow(ctx, `
            SELECT COALESCE(MAX(created_at), '1970-01-01')
            FROM sync_logs
            WHERE user_id = $1 AND direction = 'export' AND status = 'completed'
        `, userID).Scan(&lastSync)
        
        if time.Since(lastSync) > time.Duration(interval)*time.Second {
            go func(uid uuid.UUID) {
                log.Printf("🔄 Запуск автоматической синхронизации для пользователя %s", uid)
                syncProductsTo1C(uid)
                syncOrdersTo1C(uid)
            }(userID)
        }
    }
}

// syncProductsTo1C - синхронизация товаров с 1С
func syncProductsTo1C(userID uuid.UUID) {
    ctx := context.Background()
    
    var webhookURL string
    err := database.Pool.QueryRow(ctx, `
        SELECT webhook_url FROM integration_settings WHERE user_id = $1 AND integration_type = '1c'
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        return
    }
    
    rows, err := database.Pool.Query(ctx, `
        SELECT id, name, sku, price, quantity, updated_at
        FROM products
        WHERE user_id = $1 AND active = true
        AND (synced_1c = false OR synced_1c_at < updated_at)
        LIMIT 100
    `, userID)
    
    if err != nil {
        log.Printf("Ошибка получения товаров для синхронизации: %v", err)
        return
    }
    defer rows.Close()
    
    var products []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var name, sku string
        var price float64
        var quantity int
        var updatedAt time.Time
        
        rows.Scan(&id, &name, &sku, &price, &quantity, &updatedAt)
        
        products = append(products, map[string]interface{}{
            "id":       id.String(),
            "name":     name,
            "sku":      sku,
            "price":    price,
            "quantity": quantity,
        })
    }
    
    if len(products) == 0 {
        return
    }
    
    data := map[string]interface{}{
        "action":    "sync_products",
        "products":  products,
        "timestamp": time.Now().Unix(),
    }
    
    jsonData, _ := json.Marshal(data)
    
    resp, err := http.Post(webhookURL+"/sync/products", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        log.Printf("Ошибка отправки в 1С: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 200 {
        for _, p := range products {
            id, _ := uuid.Parse(p["id"].(string))
            database.Pool.Exec(ctx, `
                UPDATE products SET synced_1c = true, synced_1c_at = NOW()
                WHERE id = $1
            `, id)
        }
        
        logID := uuid.New()
        database.Pool.Exec(ctx, `
            INSERT INTO sync_logs (id, user_id, direction, entity_type, record_count, status, created_at)
            VALUES ($1, $2, 'export', 'products', $3, 'completed', NOW())
        `, logID, userID, len(products))
        
        log.Printf("✅ Синхронизировано %d товаров для пользователя %s", len(products), userID)
    }
}

// syncOrdersTo1C - синхронизация заказов с 1С
func syncOrdersTo1C(userID uuid.UUID) {
    ctx := context.Background()
    
    var webhookURL string
    err := database.Pool.QueryRow(ctx, `
        SELECT webhook_url FROM integration_settings WHERE user_id = $1 AND integration_type = '1c'
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        return
    }
    
    rows, err := database.Pool.Query(ctx, `
        SELECT id, order_number, customer_name, customer_phone, customer_email, 
               total_amount, created_at, status
        FROM orders
        WHERE user_id = $1 AND synced_1c = false
        LIMIT 50
    `, userID)
    
    if err != nil {
        log.Printf("Ошибка получения заказов для синхронизации: %v", err)
        return
    }
    defer rows.Close()
    
    var orders []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var orderNumber, customerName, customerPhone, customerEmail, status string
        var totalAmount float64
        var createdAt time.Time
        
        rows.Scan(&id, &orderNumber, &customerName, &customerPhone, &customerEmail,
            &totalAmount, &createdAt, &status)
        
        itemsRows, _ := database.Pool.Query(ctx, `
            SELECT product_name, sku, quantity, price, total
            FROM order_items
            WHERE order_id = $1
        `, id)
        
        var items []map[string]interface{}
        for itemsRows.Next() {
            var name, sku string
            var quantity int
            var price, total float64
            itemsRows.Scan(&name, &sku, &quantity, &price, &total)
            items = append(items, map[string]interface{}{
                "name":     name,
                "sku":      sku,
                "quantity": quantity,
                "price":    price,
                "total":    total,
            })
        }
        itemsRows.Close()
        
        orders = append(orders, map[string]interface{}{
            "id":       id.String(),
            "number":   orderNumber,
            "customer": customerName,
            "phone":    customerPhone,
            "email":    customerEmail,
            "total":    totalAmount,
            "date":     createdAt.Format("2006-01-02"),
            "status":   status,
            "items":    items,
        })
    }
    
    if len(orders) == 0 {
        return
    }
    
    data := map[string]interface{}{
        "action":    "sync_orders",
        "orders":    orders,
        "timestamp": time.Now().Unix(),
    }
    
    jsonData, _ := json.Marshal(data)
    
    resp, err := http.Post(webhookURL+"/sync/orders", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        log.Printf("Ошибка отправки заказов в 1С: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 200 {
        for _, o := range orders {
            id, _ := uuid.Parse(o["id"].(string))
            database.Pool.Exec(ctx, `
                UPDATE orders SET synced_1c = true, synced_1c_at = NOW()
                WHERE id = $1
            `, id)
        }
        
        logID := uuid.New()
        database.Pool.Exec(ctx, `
            INSERT INTO sync_logs (id, user_id, direction, entity_type, record_count, status, created_at)
            VALUES ($1, $2, 'export', 'orders', $3, 'completed', NOW())
        `, logID, userID, len(orders))
        
        log.Printf("✅ Синхронизировано %d заказов для пользователя %s", len(orders), userID)
    }
}

// AddWebhookHandler - обработчик webhook от 1С
func AddWebhookHandler(c *gin.Context) {
    // Получаем user_id из параметров запроса
    userIDStr := c.Query("user_id")
    if userIDStr == "" {
        userIDStr = c.GetHeader("X-User-ID")
    }
    
    var userID uuid.UUID
    var err error
    
    if userIDStr != "" {
        userID, err = uuid.Parse(userIDStr)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id format"})
            return
        }
    } else {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
        return
    }
    
    var req struct {
        Action    string                 `json:"action"`
        Data      map[string]interface{} `json:"data"`
        Timestamp int64                  `json:"timestamp"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Логируем полученный webhook
    log.Printf("📥 Webhook от 1С:")
    log.Printf("   Action: %s", req.Action)
    log.Printf("   User: %s", userID)
    log.Printf("   Data: %v", req.Data)
    log.Printf("   Timestamp: %d", req.Timestamp)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Webhook принят",
    })
}