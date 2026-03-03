package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

type Customer struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Phone     string    `json:"phone"`
    Company   string    `json:"company"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    LastSeen  time.Time `json:"last_seen"`
}

type Deal struct {
    ID            string     `json:"id"`
    CustomerID    string     `json:"customer_id"`
    Title         string     `json:"title"`
    Value         float64    `json:"value"`
    Stage         string     `json:"stage"`
    Probability   int        `json:"probability"`
    ExpectedClose time.Time  `json:"expected_close"`
    CreatedAt     time.Time  `json:"created_at"`
    ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

// CRMHandler отображает страницу CRM
// @Summary Страница CRM
// @Description Отображает интерфейс CRM системы
// @Tags CRM
// @Produce html
// @Success 200 {string} html
// @Router /crm [get]
func CRMHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "crm.html", gin.H{
        "Title": "CRM система - SaaSPro",
    })
}

// CRMHealthHandler возвращает статус CRM
// @Summary Статус CRM
// @Description Проверка работоспособности CRM
// @Tags CRM
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/crm/health [get]
func CRMHealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "operational",
        "crm":    "online",
        "time":   time.Now().Unix(),
    })
}

// GetCustomers возвращает список клиентов
// @Summary Список клиентов
// @Description Возвращает всех клиентов с фильтрацией
// @Tags CRM
// @Produce json
// @Param status query string false "Статус клиента"
// @Param limit query int false "Лимит записей"
// @Security BearerAuth
// @Success 200 {array} Customer
// @Failure 401 {object} map[string]interface{}
// @Router /api/crm/customers [get]
func GetCustomers(c *gin.Context) {
    status := c.Query("status")
    limit := c.DefaultQuery("limit", "100")

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, email, phone, company, status, created_at, last_seen
        FROM crm_customers
        WHERE ($1 = '' OR status = $1)
        ORDER BY created_at DESC
        LIMIT $2
    `, status, limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    var customers []Customer
    for rows.Next() {
        var cst Customer
        err := rows.Scan(&cst.ID, &cst.Name, &cst.Email, &cst.Phone, &cst.Company, &cst.Status, &cst.CreatedAt, &cst.LastSeen)
        if err != nil {
            continue
        }
        customers = append(customers, cst)
    }

    c.JSON(http.StatusOK, customers)
}

// GetDeals возвращает список сделок
// @Summary Список сделок
// @Description Возвращает сделки с фильтрацией по стадии
// @Tags CRM
// @Produce json
// @Param stage query string false "Стадия сделки"
// @Security BearerAuth
// @Success 200 {array} Deal
// @Failure 401 {object} map[string]interface{}
// @Router /api/crm/deals [get]
func GetDeals(c *gin.Context) {
    stage := c.Query("stage")

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, customer_id, title, value, stage, probability, expected_close, created_at, closed_at
        FROM crm_deals
        WHERE ($1 = '' OR stage = $1)
        ORDER BY created_at DESC
    `, stage)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    var deals []Deal
    for rows.Next() {
        var d Deal
        err := rows.Scan(&d.ID, &d.CustomerID, &d.Title, &d.Value, &d.Stage, &d.Probability, &d.ExpectedClose, &d.CreatedAt, &d.ClosedAt)
        if err != nil {
            continue
        }
        deals = append(deals, d)
    }

    c.JSON(http.StatusOK, deals)
}

// CreateDeal создаёт новую сделку
// @Summary Создание сделки
// @Description Добавляет новую сделку в CRM
// @Tags CRM
// @Accept json
// @Produce json
// @Param deal body Deal true "Данные сделки"
// @Security BearerAuth
// @Success 201 {object} Deal
// @Failure 400 {object} map[string]interface{}
// @Router /api/crm/deals [post]
func CreateDeal(c *gin.Context) {
    var d Deal
    if err := c.ShouldBindJSON(&d); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO crm_deals (customer_id, title, value, stage, probability, expected_close)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at
    `, d.CustomerID, d.Title, d.Value, d.Stage, d.Probability, d.ExpectedClose).Scan(&d.ID, &d.CreatedAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusCreated, d)
}

// UpdateDealStage обновляет стадию сделки
// @Summary Обновление стадии сделки
// @Description Изменяет стадию и вероятность сделки
// @Tags CRM
// @Accept json
// @Produce json
// @Param id path string true "ID сделки"
// @Param stage body object{stage=string,probability=int} true "Новая стадия"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/crm/deals/{id} [put]
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
