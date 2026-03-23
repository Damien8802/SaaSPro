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

// Получить список товаров
func GetProducts(c *gin.Context) {
    userID := getUserID(c)
    
    category := c.Query("category")
    search := c.Query("search")
    
    query := `
        SELECT p.id, p.name, p.sku, p.barcode, p.price, p.cost, p.quantity, p.min_quantity, p.unit, p.category, p.description, p.active, p.created_at
        FROM products p
        WHERE p.user_id = $1 AND p.active = true
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if category != "" {
        query += fmt.Sprintf(" AND p.category = $%d", argIndex)
        args = append(args, category)
        argIndex++
    }
    
    if search != "" {
        query += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.sku ILIKE $%d OR p.barcode ILIKE $%d)", argIndex, argIndex, argIndex)
        args = append(args, "%"+search+"%")
        argIndex++
    }
    
    query += " ORDER BY p.name"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var products []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var name, sku, barcode, unit, category, description string
        var price, cost float64
        var quantity, minQuantity int
        var active bool
        var createdAt time.Time
        
        rows.Scan(&id, &name, &sku, &barcode, &price, &cost, &quantity, &minQuantity, &unit, &category, &description, &active, &createdAt)
        
        products = append(products, map[string]interface{}{
            "id":           id,
            "name":         name,
            "sku":          sku,
            "barcode":      barcode,
            "price":        price,
            "cost":         cost,
            "quantity":     quantity,
            "min_quantity": minQuantity,
            "unit":         unit,
            "category":     category,
            "description":  description,
            "active":       active,
            "created_at":   createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"products": products})
}

// Создать товар
func CreateProduct(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Name        string  `json:"name" binding:"required"`
        Sku         string  `json:"sku"`
        Barcode     string  `json:"barcode"`
        Price       float64 `json:"price" binding:"required"`
        Cost        float64 `json:"cost"`
        Quantity    int     `json:"quantity"`
        MinQuantity int     `json:"min_quantity"`
        Unit        string  `json:"unit"`
        Category    string  `json:"category"`
        Description string  `json:"description"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.Unit == "" {
        req.Unit = "шт"
    }
    
    var productID uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO products (user_id, name, sku, barcode, price, cost, quantity, min_quantity, unit, category, description)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id
    `, userID, req.Name, req.Sku, req.Barcode, req.Price, req.Cost, req.Quantity, req.MinQuantity, req.Unit, req.Category, req.Description).Scan(&productID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "product_id": productID,
        "message":    "Товар успешно создан",
    })
}

// Обновить товар
func UpdateProduct(c *gin.Context) {
    userID := getUserID(c)
    productID := c.Param("id")
    
    var req struct {
        Name        string  `json:"name"`
        Sku         string  `json:"sku"`
        Barcode     string  `json:"barcode"`
        Price       float64 `json:"price"`
        Cost        float64 `json:"cost"`
        Quantity    int     `json:"quantity"`
        MinQuantity int     `json:"min_quantity"`
        Unit        string  `json:"unit"`
        Category    string  `json:"category"`
        Description string  `json:"description"`
        Active      bool    `json:"active"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE products 
        SET name = $1, sku = $2, barcode = $3, price = $4, cost = $5, quantity = $6, min_quantity = $7, unit = $8, category = $9, description = $10, active = $11, updated_at = NOW()
        WHERE id = $12 AND user_id = $13
    `, req.Name, req.Sku, req.Barcode, req.Price, req.Cost, req.Quantity, req.MinQuantity, req.Unit, req.Category, req.Description, req.Active, productID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Товар обновлен"})
}

// Удалить товар
func DeleteProduct(c *gin.Context) {
    userID := getUserID(c)
    productID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE products SET active = false, updated_at = NOW()
        WHERE id = $1 AND user_id = $2
    `, productID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Товар удален"})
}

// Получить список заказов
func GetOrders(c *gin.Context) {
    userID := getUserID(c)
    
    status := c.Query("status")
    
    query := `
        SELECT id, order_number, customer_name, customer_phone, customer_email, total_amount, status, payment_status, created_at
        FROM orders
        WHERE user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if status != "" {
        query += fmt.Sprintf(" AND status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }
    
    query += " ORDER BY created_at DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var orders []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var orderNumber, customerName, customerPhone, customerEmail, status, paymentStatus string
        var totalAmount float64
        var createdAt time.Time
        
        rows.Scan(&id, &orderNumber, &customerName, &customerPhone, &customerEmail, &totalAmount, &status, &paymentStatus, &createdAt)
        
        orders = append(orders, map[string]interface{}{
            "id":              id,
            "order_number":    orderNumber,
            "customer_name":   customerName,
            "customer_phone":  customerPhone,
            "customer_email":  customerEmail,
            "total_amount":    totalAmount,
            "status":          status,
            "payment_status":  paymentStatus,
            "created_at":      createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// Создать заказ
func CreateOrder(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        CustomerName    string `json:"customer_name" binding:"required"`
        CustomerPhone   string `json:"customer_phone"`
        CustomerEmail   string `json:"customer_email"`
        DeliveryAddress string `json:"delivery_address"`
        Notes           string `json:"notes"`
        Items           []struct {
            ProductID string  `json:"product_id" binding:"required"`
            Quantity  int     `json:"quantity" binding:"required"`
            Price     float64 `json:"price"`
        } `json:"items" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Генерируем номер заказа
    orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano()%1000000)
    
    var totalAmount float64
    var items []struct {
        ProductID uuid.UUID
        Name      string
        Sku       string
        Quantity  int
        Price     float64
        Total     float64
    }
    
    for _, item := range req.Items {
        // Получаем информацию о товаре
        var productID uuid.UUID
        var name, sku string
        var currentPrice float64
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT id, name, sku, price FROM products 
            WHERE id = $1 AND user_id = $2 AND active = true
        `, item.ProductID, userID).Scan(&productID, &name, &sku, &currentPrice)
        
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found: " + item.ProductID})
            return
        }
        
        price := item.Price
        if price == 0 {
            price = currentPrice
        }
        
        total := price * float64(item.Quantity)
        totalAmount += total
        
        items = append(items, struct {
            ProductID uuid.UUID
            Name      string
            Sku       string
            Quantity  int
            Price     float64
            Total     float64
        }{productID, name, sku, item.Quantity, price, total})
    }
    
    // Создаем заказ
    var orderID uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO orders (user_id, order_number, customer_name, customer_phone, customer_email, total_amount, delivery_address, notes)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id
    `, userID, orderNumber, req.CustomerName, req.CustomerPhone, req.CustomerEmail, totalAmount, req.DeliveryAddress, req.Notes).Scan(&orderID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
        return
    }
    
    // Добавляем позиции и обновляем остатки
    for _, item := range items {
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO order_items (order_id, product_id, product_name, sku, quantity, price, total)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, orderID, item.ProductID, item.Name, item.Sku, item.Quantity, item.Price, item.Total)
        
        if err != nil {
            log.Printf("Failed to add order item: %v", err)
        }
        
        // Уменьшаем остаток
        database.Pool.Exec(c.Request.Context(), `
            UPDATE products SET quantity = quantity - $1, updated_at = NOW()
            WHERE id = $2 AND user_id = $3
        `, item.Quantity, item.ProductID, userID)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "order_id":     orderID,
        "order_number": orderNumber,
        "total":        totalAmount,
        "message":      "Заказ успешно создан",
    })
}

// Получить детали заказа
func GetOrderDetails(c *gin.Context) {
    userID := getUserID(c)
    orderID := c.Param("id")
    
    var order struct {
        ID              uuid.UUID
        OrderNumber     string
        CustomerName    string
        CustomerPhone   string
        CustomerEmail   string
        TotalAmount     float64
        Status          string
        PaymentStatus   string
        DeliveryAddress string
        Notes           string
        CreatedAt       time.Time
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, order_number, customer_name, customer_phone, customer_email, total_amount, status, payment_status, delivery_address, notes, created_at
        FROM orders
        WHERE id = $1 AND user_id = $2
    `, orderID, userID).Scan(&order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone, &order.CustomerEmail, &order.TotalAmount, &order.Status, &order.PaymentStatus, &order.DeliveryAddress, &order.Notes, &order.CreatedAt)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }
    
    // Получаем позиции
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT product_name, sku, quantity, price, total
        FROM order_items
        WHERE order_id = $1
    `, orderID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get items"})
        return
    }
    defer rows.Close()
    
    var items []map[string]interface{}
    for rows.Next() {
        var name, sku string
        var quantity int
        var price, total float64
        
        rows.Scan(&name, &sku, &quantity, &price, &total)
        
        items = append(items, map[string]interface{}{
            "name":     name,
            "sku":      sku,
            "quantity": quantity,
            "price":    price,
            "total":    total,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "order": order,
        "items": items,
    })
}

// Получить статистику склада
func GetInventoryStats(c *gin.Context) {
    userID := getUserID(c)
    
    var stats struct {
        TotalProducts   int     `json:"total_products"`
        TotalValue      float64 `json:"total_value"`
        LowStockCount   int     `json:"low_stock_count"`
        OutOfStockCount int     `json:"out_of_stock_count"`
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COUNT(*) as total_products,
            COALESCE(SUM(price * quantity), 0) as total_value,
            COUNT(CASE WHEN quantity <= min_quantity AND quantity > 0 THEN 1 END) as low_stock,
            COUNT(CASE WHEN quantity = 0 THEN 1 END) as out_of_stock
        FROM products
        WHERE user_id = $1 AND active = true
    `, userID).Scan(&stats.TotalProducts, &stats.TotalValue, &stats.LowStockCount, &stats.OutOfStockCount)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// Страница инвентаризации
func InventoryPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "inventory.html", gin.H{
        "title": "Складской учет | SaaSPro",
    })
}