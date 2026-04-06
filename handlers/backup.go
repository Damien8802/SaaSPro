package handlers

import (
 "database/sql"    
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

type BackupData struct {
    Customers    []Customer    `json:"customers"`
    Deals        []Deal        `json:"deals"`
    Tags         []Tag         `json:"tags"`
    Activities   []Activity    `json:"activities"`
    BackupDate   time.Time     `json:"backup_date"`
}

// CreateBackup создает JSON бэкап всех данных CRM
func CreateBackup(c *gin.Context) {
    backup := BackupData{
        BackupDate: time.Now(),
    }
    
    // Получаем клиентов
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, email, phone, company, status, responsible, 
               source, comment, user_id, lead_score, created_at, last_seen,
               city, social_media, birthday, notes
        FROM crm_customers
    `)
    if err == nil {
        for rows.Next() {
            var cst Customer
            var socialMedia []byte
            var birthday sql.NullTime
            rows.Scan(&cst.ID, &cst.Name, &cst.Email, &cst.Phone, &cst.Company,
                &cst.Status, &cst.Responsible, &cst.Source, &cst.Comment, &cst.UserID,
                &cst.LeadScore, &cst.CreatedAt, &cst.LastSeen, &cst.City,
                &socialMedia, &birthday, &cst.Notes)
            if birthday.Valid {
                cst.Birthday = &birthday.Time
            }
            backup.Customers = append(backup.Customers, cst)
        }
        rows.Close()
    }
    
    // Получаем сделки
    rows, err = database.Pool.Query(c.Request.Context(), `
        SELECT id, customer_id, title, value, stage, probability, responsible,
               source, comment, user_id, expected_close, created_at, closed_at,
               product_category, discount, next_action_date
        FROM crm_deals
    `)
    if err == nil {
        for rows.Next() {
            var d Deal
            var nextActionDate sql.NullTime
            rows.Scan(&d.ID, &d.CustomerID, &d.Title, &d.Value, &d.Stage,
                &d.Probability, &d.Responsible, &d.Source, &d.Comment, &d.UserID,
                &d.ExpectedClose, &d.CreatedAt, &d.ClosedAt, &d.ProductCategory,
                &d.Discount, &nextActionDate)
            if nextActionDate.Valid {
                d.NextActionDate = &nextActionDate.Time
            }
            backup.Deals = append(backup.Deals, d)
        }
        rows.Close()
    }
    
    // Получаем теги
    rows, err = database.Pool.Query(c.Request.Context(), `
        SELECT id, name, color, created_at FROM tags
    `)
    if err == nil {
        for rows.Next() {
            var t Tag
            rows.Scan(&t.ID, &t.Name, &t.Color, &t.CreatedAt)
            backup.Tags = append(backup.Tags, t)
        }
        rows.Close()
    }
    
    // Конвертируем в JSON
    jsonData, err := json.MarshalIndent(backup, "", "  ")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания бэкапа"})
        return
    }
    
    // Отправляем файл
    filename := fmt.Sprintf("crm_backup_%s.json", time.Now().Format("20060102_150405"))
    c.Header("Content-Disposition", "attachment; filename="+filename)
    c.Header("Content-Type", "application/json")
    c.Data(http.StatusOK, "application/json", jsonData)
}

// RestoreBackup восстанавливает данные из JSON бэкапа
func RestoreBackup(c *gin.Context) {
    file, err := c.FormFile("backup")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не загружен"})
        return
    }
    
    src, err := file.Open()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения файла"})
        return
    }
    defer src.Close()
    
    var backup BackupData
    decoder := json.NewDecoder(src)
    if err := decoder.Decode(&backup); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат бэкапа"})
        return
    }
    
    // Начинаем транзакцию
    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка БД"})
        return
    }
    defer tx.Rollback(c.Request.Context())
    
    // Восстанавливаем данные (очищаем старые)
    tx.Exec(c.Request.Context(), "TRUNCATE crm_deals, crm_customers, tags, activities CASCADE")
    
    // Восстанавливаем клиентов
    for _, cust := range backup.Customers {
        tx.Exec(c.Request.Context(), `
            INSERT INTO crm_customers (id, name, email, phone, company, status, 
                responsible, source, comment, user_id, lead_score, created_at, 
                last_seen, city, social_media, birthday, notes)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
        `, cust.ID, cust.Name, cust.Email, cust.Phone, cust.Company, cust.Status,
            cust.Responsible, cust.Source, cust.Comment, cust.UserID, cust.LeadScore,
            cust.CreatedAt, cust.LastSeen, cust.City, cust.SocialMedia, cust.Birthday, cust.Notes)
    }
    
    // Восстанавливаем сделки
    for _, deal := range backup.Deals {
        tx.Exec(c.Request.Context(), `
            INSERT INTO crm_deals (id, customer_id, title, value, stage, probability,
                responsible, source, comment, user_id, expected_close, created_at,
                closed_at, product_category, discount, next_action_date)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
        `, deal.ID, deal.CustomerID, deal.Title, deal.Value, deal.Stage, deal.Probability,
            deal.Responsible, deal.Source, deal.Comment, deal.UserID, deal.ExpectedClose,
            deal.CreatedAt, deal.ClosedAt, deal.ProductCategory, deal.Discount, deal.NextActionDate)
    }
    
    // Восстанавливаем теги
    for _, tag := range backup.Tags {
        tx.Exec(c.Request.Context(), `
            INSERT INTO tags (id, name, color, created_at)
            VALUES ($1, $2, $3, $4)
        `, tag.ID, tag.Name, tag.Color, tag.CreatedAt)
    }
    
    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка восстановления"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Бэкап восстановлен"})
}