package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// ==================== AVITO INTEGRATION ====================

type AvitoAuthRequest struct {
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
    Username     string `json:"username"`
    Password     string `json:"password"`
}

type AvitoTokenResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    ExpiresIn   int    `json:"expires_in"`
}

type AvitoVacancyRequest struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    CategoryID  int    `json:"category_id"`
    Salary      int    `json:"salary"`
    City        string `json:"city"`
    Phone       string `json:"phone"`
    Email       string `json:"email"`
}

// Публикация в Avito
func PublishToAvito(c *gin.Context) {
    var req struct {
        VacancyID    string `json:"vacancy_id" binding:"required"`
        Email        string `json:"email" binding:"required"`
        Password     string `json:"password" binding:"required"`
        Phone        string `json:"phone"`
        City         string `json:"city"`
        CategoryID   int    `json:"category_id"`
        Remember     bool   `json:"remember"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    // Получаем данные вакансии
    var vacancy struct {
        ID          string
        Title       string
        Description string
        SalaryMin   float64
        SalaryMax   float64
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, title, description, salary_min, salary_max
        FROM hr_vacancies WHERE id = $1 AND tenant_id = $2
    `, req.VacancyID, tenantID).Scan(
        &vacancy.ID, &vacancy.Title, &vacancy.Description,
        &vacancy.SalaryMin, &vacancy.SalaryMax,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Вакансия не найдена"})
        return
    }
    
    // Авторизация в Avito
    token, err := authenticateAvito(req.Email, req.Password)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Ошибка авторизации в Avito: " + err.Error()})
        return
    }
    
    // Формируем запрос
    avitoVacancy := AvitoVacancyRequest{
        Title:       vacancy.Title,
        Description: vacancy.Description,
        CategoryID:  req.CategoryID,
        Salary:      int(vacancy.SalaryMax),
        City:        req.City,
        Phone:       req.Phone,
        Email:       req.Email,
    }
    
    if avitoVacancy.CategoryID == 0 {
        avitoVacancy.CategoryID = 74 // IT категория по умолчанию
    }
    
    jsonData, _ := json.Marshal(avitoVacancy)
    
    // Отправляем запрос в Avito API
    avitoReq, _ := http.NewRequest("POST", "https://api.avito.ru/job/v1/items", bytes.NewBuffer(jsonData))
    avitoReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
    avitoReq.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(avitoReq)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка связи с Avito"})
        return
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    if resp.StatusCode != 200 && resp.StatusCode != 201 {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":  "Ошибка публикации в Avito",
            "status": resp.StatusCode,
            "body":   string(body),
        })
        return
    }
    
    var avitoResp struct {
        ID string `json:"id"`
    }
    json.Unmarshal(body, &avitoResp)
    
    // Сохраняем связь
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO hr_platform_vacancies (vacancy_id, platform, platform_vacancy_id, status, published_at, tenant_id)
        VALUES ($1, 'avito', $2, 'published', NOW(), $3)
        ON CONFLICT (vacancy_id, platform) DO UPDATE 
        SET platform_vacancy_id = $2, status = 'published', published_at = NOW(), updated_at = NOW()
    `, vacancy.ID, avitoResp.ID, tenantID)
    
    if err != nil {
        log.Printf("Ошибка сохранения связи: %v", err)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Вакансия успешно опубликована в Avito",
        "url":     fmt.Sprintf("https://www.avito.ru/job/%s", avitoResp.ID),
    })
}

// Авторизация в Avito
func authenticateAvito(username, password string) (*AvitoTokenResponse, error) {
    data := map[string]string{
        "username": username,
        "password": password,
        "grant_type": "password",
    }
    
    jsonData, _ := json.Marshal(data)
    
    resp, err := http.Post("https://api.avito.ru/auth/login", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("ошибка авторизации: %s", string(body))
    }
    
    var token AvitoTokenResponse
    json.Unmarshal(body, &token)
    
    return &token, nil
}

// ==================== РАБОТА.РУ INTEGRATION ====================

type RabotaRuVacancyRequest struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    SalaryFrom  int    `json:"salary_from"`
    SalaryTo    int    `json:"salary_to"`
    City        string `json:"city"`
    Email       string `json:"email"`
    Phone       string `json:"phone"`
}

// Публикация на Работа.ру (через партнёрский API или экспорт)
func PublishToRabotaRu(c *gin.Context) {
    var req struct {
        VacancyID    string `json:"vacancy_id" binding:"required"`
        Email        string `json:"email" binding:"required"`
        Password     string `json:"password" binding:"required"`
        Phone        string `json:"phone"`
        City         string `json:"city"`
        Remember     bool   `json:"remember"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    // Получаем данные вакансии
    var vacancy struct {
        ID          string
        Title       string
        Description string
        SalaryMin   float64
        SalaryMax   float64
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, title, description, salary_min, salary_max
        FROM hr_vacancies WHERE id = $1 AND tenant_id = $2
    `, req.VacancyID, tenantID).Scan(
        &vacancy.ID, &vacancy.Title, &vacancy.Description,
        &vacancy.SalaryMin, &vacancy.SalaryMax,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Вакансия не найдена"})
        return
    }
    
    // Формируем данные для экспорта (т.к. прямого API нет)
    exportData := RabotaRuVacancyRequest{
        Title:       vacancy.Title,
        Description: vacancy.Description,
        SalaryFrom:  int(vacancy.SalaryMin),
        SalaryTo:    int(vacancy.SalaryMax),
        City:        req.City,
        Email:       req.Email,
        Phone:       req.Phone,
    }
    
    // Сохраняем в БД как экспортированную
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO hr_platform_vacancies (vacancy_id, platform, status, published_at, tenant_id, export_data)
        VALUES ($1, 'rabota', 'exported', NOW(), $2, $3)
        ON CONFLICT (vacancy_id, platform) DO UPDATE 
        SET status = 'exported', published_at = NOW(), export_data = $3, updated_at = NOW()
    `, vacancy.ID, tenantID, exportData)
    
    if err != nil {
        log.Printf("Ошибка сохранения: %v", err)
    }
    
    // Возвращаем ссылку на экспорт
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Данные вакансии подготовлены для экспорта на Работа.ру",
        "export_url": fmt.Sprintf("/api/hh/export/%s/rabota", vacancy.ID),
    })
}

// Получение статуса публикации на платформе
func GetPlatformPublishStatus(c *gin.Context) {
    vacancyID := c.Param("id")
    platform := c.Param("platform")
    tenantID := c.GetString("tenant_id")
    
    var platformVacancyID string
    var status string
    var publishedAt time.Time
    var exportData json.RawMessage
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT platform_vacancy_id, status, published_at, export_data
        FROM hr_platform_vacancies 
        WHERE vacancy_id = $1 AND platform = $2 AND tenant_id = $3
    `, vacancyID, platform, tenantID).Scan(&platformVacancyID, &status, &publishedAt, &exportData)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "published": false,
            "status":    "not_published",
        })
        return
    }
    
    response := gin.H{
        "published": status == "published" || status == "exported",
        "status":    status,
        "published_at": publishedAt,
    }
    
    if platformVacancyID != "" {
        if platform == "avito" {
            response["url"] = fmt.Sprintf("https://www.avito.ru/job/%s", platformVacancyID)
        }
    }
    
    c.JSON(http.StatusOK, response)
}