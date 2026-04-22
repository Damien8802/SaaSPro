package services

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type YandexHRAssistant struct {
    db       *pgxpool.Pool
    apiKey   string
    folderID string
}

func NewYandexHRAssistant(db *pgxpool.Pool, apiKey, folderID string) *YandexHRAssistant {
    return &YandexHRAssistant{
        db:       db,
        apiKey:   apiKey,
        folderID: folderID,
    }
}

// Process - главный метод
func (y *YandexHRAssistant) Process(tenantID, userID uuid.UUID, message string) (string, error) {
    // Формируем промпт для YandexGPT
    prompt := fmt.Sprintf(`Ты - HR ассистент. Пользователь написал: "%s"

Определи, что хочет пользователь, и ответь ТОЛЬКО в формате JSON:
{
    "action": "create_vacancy",
    "data": {
        "title": "название вакансии"
    },
    "reply": "твой ответ пользователю"
}

Доступные действия: create_vacancy, list_vacancies, publish_vacancy, hire_employee, fire_employee, give_bonus, vacation, stats, help

Отвечай ТОЛЬКО JSON. Никакого другого текста.`, message)

    // Вызываем YandexGPT
    response, err := y.callYandexGPT(prompt)
    if err != nil {
        return y.getHelp(), nil
    }
    
    // Парсим JSON
    var result struct {
        Action string `json:"action"`
        Data   struct {
            Title      string  `json:"title"`
            Department string  `json:"department"`
            SalaryFrom float64 `json:"salary_from"`
            SalaryTo   float64 `json:"salary_to"`
            Platform   string  `json:"platform"`
            Name       string  `json:"name"`
            Position   string  `json:"position"`
            Amount     float64 `json:"amount"`
            Dates      string  `json:"dates"`
        } `json:"data"`
        Reply string `json:"reply"`
    }
    
    if err := json.Unmarshal([]byte(response), &result); err != nil {
        return y.getHelp(), nil
    }
    
    // Выполняем действие
    switch result.Action {
    case "create_vacancy":
        return y.createVacancy(tenantID, result.Data.Title)
    case "list_vacancies":
        return y.listVacancies(tenantID)
    case "stats":
        return y.getStats(tenantID)
    default:
        if result.Reply != "" {
            return result.Reply, nil
        }
        return y.getHelp(), nil
    }
}

// callYandexGPT - вызов API
func (y *YandexHRAssistant) callYandexGPT(prompt string) (string, error) {
    url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
    
    reqBody := map[string]interface{}{
        "modelUri": fmt.Sprintf("gpt://%s/yandexgpt-lite", y.folderID),
        "completionOptions": map[string]interface{}{
            "stream":      false,
            "temperature": 0.3,
            "maxTokens":   500,
        },
        "messages": []map[string]string{
            {"role": "user", "text": prompt},
        },
    }
    
    jsonData, _ := json.Marshal(reqBody)
    
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Api-Key %s", y.apiKey))
    
    client := &http.Client{Timeout: 15 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    var result map[string]interface{}
    json.Unmarshal(body, &result)
    
    // Извлекаем текст
    if alternatives, ok := result["result"].(map[string]interface{}); ok {
        if altList, ok := alternatives["alternatives"].([]interface{}); ok && len(altList) > 0 {
            if msg, ok := altList[0].(map[string]interface{}); ok {
                if message, ok := msg["message"].(map[string]interface{}); ok {
                    if text, ok := message["text"].(string); ok {
                        return text, nil
                    }
                }
            }
        }
    }
    
    return "", fmt.Errorf("пустой ответ")
}

// createVacancy - создание вакансии
func (y *YandexHRAssistant) createVacancy(tenantID uuid.UUID, title string) (string, error) {
    if title == "" {
        return "📝 **Укажите название вакансии**\n\nПример: создай вакансию разработчик", nil
    }
    
    var id string
    err := y.db.QueryRow(context.Background(), `
        INSERT INTO hr_vacancies (title, status, tenant_id, created_at)
        VALUES ($1, 'open', $2, NOW())
        RETURNING id::text
    `, title, tenantID).Scan(&id)
    
    if err != nil {
        return "❌ Ошибка создания вакансии", nil
    }
    
    return fmt.Sprintf("✅ **Вакансия создана!**\n📌 %s\n🆔 ID: %s", title, id[:8]), nil
}

// listVacancies - список вакансий
func (y *YandexHRAssistant) listVacancies(tenantID uuid.UUID) (string, error) {
    rows, err := y.db.Query(context.Background(), `
        SELECT title FROM hr_vacancies WHERE tenant_id = $1 AND status = 'open' LIMIT 10
    `, tenantID)
    if err != nil {
        return "❌ Ошибка", nil
    }
    defer rows.Close()
    
    var list []string
    for rows.Next() {
        var title string
        rows.Scan(&title)
        list = append(list, "• "+title)
    }
    
    if len(list) == 0 {
        return "📭 **Нет вакансий**\n\nСоздайте: создай вакансию разработчик", nil
    }
    
    return "📋 **Ваши вакансии:**\n" + strings.Join(list, "\n"), nil
}

// getStats - статистика
func (y *YandexHRAssistant) getStats(tenantID uuid.UUID) (string, error) {
    var employees, vacancies int
    y.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM hr_employees WHERE tenant_id=$1 AND status='active'", tenantID).Scan(&employees)
    y.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM hr_vacancies WHERE tenant_id=$1 AND status='open'", tenantID).Scan(&vacancies)
    
    return fmt.Sprintf("📊 **HR Статистика**\n\n👥 Сотрудников: %d\n💼 Вакансий: %d", employees, vacancies), nil
}

// getHelp - справка
func (y *YandexHRAssistant) getHelp() string {
    return `🤖 **Я HR ассистент**

Что я могу:
• создать вакансию разработчик
• список вакансий
• статистика

Просто напишите, что нужно сделать!`
}