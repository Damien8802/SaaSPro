package handlers

import (
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// Customer представляет клиента в CRM
type Customer struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Email       string    `json:"email"`
    Phone       string    `json:"phone"`
    Company     string    `json:"company"`
    Status      string    `json:"status"`
    Responsible string    `json:"responsible"`
    Source      string    `json:"source"`
    Comment     string    `json:"comment"`
    CreatedAt   time.Time `json:"created_at"`
    LastSeen    time.Time `json:"last_seen"`
}

// Deal представляет сделку в CRM
type Deal struct {
    ID            string     `json:"id"`
    CustomerID    string     `json:"customer_id"`
    Title         string     `json:"title"`
    Value         float64    `json:"value"`
    Stage         string     `json:"stage"`
    Probability   int        `json:"probability"`
    Responsible   string     `json:"responsible"`
    Source        string     `json:"source"`
    Comment       string     `json:"comment"`
    ExpectedClose time.Time  `json:"expected_close"`
    CreatedAt     time.Time  `json:"created_at"`
    ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

// getPaginationParams извлекает page и page_size из запроса с значениями по умолчанию
func getPaginationParams(c *gin.Context) (page, pageSize int) {
    page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
    if err != nil || page < 1 {
        page = 1
    }
    pageSize, err = strconv.Atoi(c.DefaultQuery("page_size", "20"))
    if err != nil || pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }
    return page, pageSize
}

// ========== СТРАНИЦА CRM ==========

// CRMHandler отображает страницу CRM
func CRMHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "crm.html", gin.H{
        "Title": "CRM система - SaaSPro",
    })
}

// CRMHealthHandler возвращает статус CRM
func CRMHealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "operational",
        "crm":    "online",
        "time":   time.Now().Unix(),
    })
}

// ========== КЛИЕНТЫ ==========

// GetCustomers возвращает список клиентов с пагинацией, поиском и фильтрацией по дате создания
func GetCustomers(c *gin.Context) {
    status := c.Query("status")
    search := c.Query("search")
    createdFrom := c.Query("created_from") // NEW: дата начала (YYYY-MM-DD)
    createdTo := c.Query("created_to")     // NEW: дата окончания
    page, pageSize := getPaginationParams(c)
    offset := (page - 1) * pageSize

    // ---- Подсчёт общего количества записей с учётом всех фильтров ----
    countQuery := `SELECT COUNT(*) FROM crm_customers`
    countArgs := []interface{}{}
    whereClause := ""

    // Фильтр по статусу
    if status != "" {
        whereClause += " status = $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, status)
    }
    // Фильтр по поиску
    if search != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " (name ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%' OR email ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%')"
        countArgs = append(countArgs, search)
    }
    // Фильтр по дате создания (от)
    if createdFrom != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " created_at >= $" + strconv.Itoa(len(countArgs)+1) + "::date"
        countArgs = append(countArgs, createdFrom)
    }
    // Фильтр по дате создания (до)
    if createdTo != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " created_at < ($" + strconv.Itoa(len(countArgs)+1) + "::date + '1 day'::interval)"
        countArgs = append(countArgs, createdTo)
    }

    if whereClause != "" {
        countQuery += " WHERE" + whereClause
    }

    var total int
    err := database.Pool.QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // ---- Запрос данных с теми же фильтрами и пагинацией ----
    query := `SELECT id, name, email, phone, company, status, responsible, source, comment, created_at, last_seen
              FROM crm_customers`
    args := []interface{}{}
    whereData := ""

    if status != "" {
        whereData += " status = $" + strconv.Itoa(len(args)+1)
        args = append(args, status)
    }
    if search != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " (name ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%' OR email ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%')"
        args = append(args, search)
    }
    if createdFrom != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " created_at >= $" + strconv.Itoa(len(args)+1) + "::date"
        args = append(args, createdFrom)
    }
    if createdTo != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " created_at < ($" + strconv.Itoa(len(args)+1) + "::date + '1 day'::interval)"
        args = append(args, createdTo)
    }

    if whereData != "" {
        query += " WHERE" + whereData
    }
    query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
    args = append(args, pageSize, offset)

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    customers := make([]Customer, 0)
    for rows.Next() {
        var cst Customer
        err := rows.Scan(&cst.ID, &cst.Name, &cst.Email, &cst.Phone, &cst.Company, &cst.Status,
            &cst.Responsible, &cst.Source, &cst.Comment, &cst.CreatedAt, &cst.LastSeen)
        if err != nil {
            continue
        }
        customers = append(customers, cst)
    }

    c.JSON(http.StatusOK, gin.H{
        "data":        customers,
        "total":       total,
        "page":        page,
        "page_size":   pageSize,
        "total_pages": (total + pageSize - 1) / pageSize,
    })
}

// CreateCustomer создаёт нового клиента
func CreateCustomer(c *gin.Context) {
    var req Customer
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var id string
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO crm_customers (name, email, phone, company, status, responsible, source, comment, created_at, last_seen)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING id
    `, req.Name, req.Email, req.Phone, req.Company, req.Status,
        req.Responsible, req.Source, req.Comment).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"id": id})
}

// UpdateCustomer обновляет данные клиента
func UpdateCustomer(c *gin.Context) {
    id := c.Param("id")
    var req Customer
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_customers
        SET name = $1, email = $2, phone = $3, company = $4, status = $5,
            responsible = $6, source = $7, comment = $8, last_seen = NOW()
        WHERE id = $9
    `, req.Name, req.Email, req.Phone, req.Company, req.Status,
        req.Responsible, req.Source, req.Comment, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteCustomer удаляет клиента
func DeleteCustomer(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), "DELETE FROM crm_customers WHERE id = $1", id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== СДЕЛКИ ==========

// GetDeals возвращает список сделок с пагинацией, поиском и фильтрацией по сумме и дате закрытия
func GetDeals(c *gin.Context) {
    stage := c.Query("stage")
    search := c.Query("search")
    valueMin := c.Query("value_min")   // NEW: минимальная сумма
    valueMax := c.Query("value_max")   // NEW: максимальная сумма
    closeFrom := c.Query("close_from") // NEW: ожидаемая дата закрытия от
    closeTo := c.Query("close_to")     // NEW: ожидаемая дата закрытия до
    page, pageSize := getPaginationParams(c)
    offset := (page - 1) * pageSize

    // ---- Подсчёт общего количества с учётом всех фильтров ----
    countQuery := `SELECT COUNT(*) FROM crm_deals`
    countArgs := []interface{}{}
    whereClause := ""

    if stage != "" {
        whereClause += " stage = $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, stage)
    }
    if search != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " title ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%'"
        countArgs = append(countArgs, search)
    }
    if valueMin != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " value >= $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, valueMin)
    }
    if valueMax != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " value <= $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, valueMax)
    }
    if closeFrom != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " expected_close >= $" + strconv.Itoa(len(countArgs)+1) + "::date"
        countArgs = append(countArgs, closeFrom)
    }
    if closeTo != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " expected_close < ($" + strconv.Itoa(len(countArgs)+1) + "::date + '1 day'::interval)"
        countArgs = append(countArgs, closeTo)
    }

    if whereClause != "" {
        countQuery += " WHERE" + whereClause
    }

    var total int
    err := database.Pool.QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // ---- Запрос данных с теми же фильтрами ----
    query := `SELECT id, customer_id, title, value, stage, probability, responsible, source, comment, expected_close, created_at, closed_at
              FROM crm_deals`
    args := []interface{}{}
    whereData := ""

    if stage != "" {
        whereData += " stage = $" + strconv.Itoa(len(args)+1)
        args = append(args, stage)
    }
    if search != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " title ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%'"
        args = append(args, search)
    }
    if valueMin != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " value >= $" + strconv.Itoa(len(args)+1)
        args = append(args, valueMin)
    }
    if valueMax != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " value <= $" + strconv.Itoa(len(args)+1)
        args = append(args, valueMax)
    }
    if closeFrom != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " expected_close >= $" + strconv.Itoa(len(args)+1) + "::date"
        args = append(args, closeFrom)
    }
    if closeTo != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " expected_close < ($" + strconv.Itoa(len(args)+1) + "::date + '1 day'::interval)"
        args = append(args, closeTo)
    }

    if whereData != "" {
        query += " WHERE" + whereData
    }
    query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
    args = append(args, pageSize, offset)

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    deals := make([]Deal, 0)
    for rows.Next() {
        var d Deal
        err := rows.Scan(&d.ID, &d.CustomerID, &d.Title, &d.Value, &d.Stage, &d.Probability,
            &d.Responsible, &d.Source, &d.Comment, &d.ExpectedClose, &d.CreatedAt, &d.ClosedAt)
        if err != nil {
            continue
        }
        deals = append(deals, d)
    }

    c.JSON(http.StatusOK, gin.H{
        "data":        deals,
        "total":       total,
        "page":        page,
        "page_size":   pageSize,
        "total_pages": (total + pageSize - 1) / pageSize,
    })
}

// CreateDeal создаёт новую сделку
func CreateDeal(c *gin.Context) {
    var d Deal
    if err := c.ShouldBindJSON(&d); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO crm_deals (customer_id, title, value, stage, probability, responsible, source, comment, expected_close, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        RETURNING id
    `, d.CustomerID, d.Title, d.Value, d.Stage, d.Probability,
        d.Responsible, d.Source, d.Comment, d.ExpectedClose).Scan(&d.ID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusCreated, d)
}

// UpdateDeal обновляет сделку
func UpdateDeal(c *gin.Context) {
    id := c.Param("id")
    var d Deal
    if err := c.ShouldBindJSON(&d); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_deals
        SET title = $1, value = $2, stage = $3, probability = $4,
            responsible = $5, source = $6, comment = $7, expected_close = $8, updated_at = NOW()
        WHERE id = $9
    `, d.Title, d.Value, d.Stage, d.Probability,
        d.Responsible, d.Source, d.Comment, d.ExpectedClose, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateDealStage обновляет только стадию сделки
func UpdateDealStage(c *gin.Context) {
    id := c.Param("id")
    var req struct {
        Stage       string `json:"stage"`
        Probability int    `json:"probability"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_deals
        SET stage = $1, probability = $2, updated_at = NOW()
        WHERE id = $3
    `, req.Stage, req.Probability, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteDeal удаляет сделку
func DeleteDeal(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), "DELETE FROM crm_deals WHERE id = $1", id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}