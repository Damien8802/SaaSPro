package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/gin-gonic/gin"

    "subscription-system/database"
)

type AskRequest struct {
    Question    string `json:"question" binding:"required"`
    CRMContext  bool   `json:"crm_context"`
    Recommend   bool   `json:"recommend"`
    RequestType string `json:"request_type"` // crm, deal, analytics
}

type YandexGPTRequest struct {
    ModelUri          string `json:"modelUri"`
    CompletionOptions struct {
        Stream      bool    `json:"stream"`
        Temperature float64 `json:"temperature"`
        MaxTokens   int     `json:"maxTokens"`
    } `json:"completionOptions"`
    Messages []struct {
        Role string `json:"role"`
        Text string `json:"text"`
    } `json:"messages"`
}

type YandexGPTResponse struct {
    Result struct {
        Alternatives []struct {
            Message struct {
                Role string `json:"role"`
                Text string `json:"text"`
            } `json:"message"`
        } `json:"alternatives"`
        Usage struct {
            InputTextTokens  string `json:"inputTextTokens"`
            CompletionTokens string `json:"completionTokens"`
            TotalTokens      string `json:"totalTokens"`
        } `json:"usage"`
        ModelVersion string `json:"modelVersion"`
    } `json:"result"`
}

// Типы запросов
type AIRequestType string

const (
    AIRequestCRM       AIRequestType = "crm"
    AIRequestDeal      AIRequestType = "deal"
    AIRequestAnalytics AIRequestType = "analytics"
)

// Системный промпт для неавторизованных пользователей (режим "Консультант")
const consultantPrompt = `Ты AI-консультант CRM системы SaaSPro. Твоя задача - рассказывать о возможностях системы, помогать с выбором тарифа и отвечать на общие вопросы.

📌 **О CRM системе:**
• Управление клиентами и сделками
• Аналитика и воронка продаж
• Канбан-доска для визуализации процессов
• Календарь для планирования
• AI-ассистент для рекомендаций
• Интеграции с Telegram, email
• API для разработчиков

💰 **Тарифы:**
• Бесплатный - до 10 клиентов, базовые функции
• Базовый - до 100 клиентов, расширенная аналитика
• Профессиональный - неограниченно, все функции + API
• Корпоративный - индивидуальные условия

❓ **Частые вопросы:**
• Как начать работу?
• Какие есть интеграции?
• Как подключить Telegram бота?
• Как экспортировать данные?
• Как настроить уведомления?

Отвечай дружелюбно, структурированно, с эмодзи. Если вопрос касается личных данных - предложи авторизоваться.`

// Функция для определения типа запроса
func detectAIRequestType(question string, userID string, c *gin.Context) (AIRequestType, map[string]interface{}) {
    question = strings.ToLower(question)
    result := make(map[string]interface{})
    
    // Проверяем, есть ли конкретный ID сделки в запросе
    dealID := c.Query("deal_id")
    if dealID != "" {
        result["deal_id"] = dealID
        return AIRequestDeal, result
    }
    
    // Ключевые слова для аналитики
    analyticsKeywords := []string{
        "аналитик", "статистик", "график", "воронк", "продаж", "отчет", 
        "показател", "метрик", "kpi", "динамик", "сколько", "итого", 
        "всего", "средний", "общий", "сумма всех",
    }
    for _, keyword := range analyticsKeywords {
        if strings.Contains(question, keyword) {
            return AIRequestAnalytics, result
        }
    }
    
    // Ключевые слова для сделок
    dealKeywords := []string{
        "сделк", "сумма сделки", "этап", "стади", "вероятност", 
        "переговор", "закрыт", "успешн", "проигран", "лид", "клиент",
    }
    for _, keyword := range dealKeywords {
        if strings.Contains(question, keyword) {
            return AIRequestDeal, result
        }
    }
    
    // По умолчанию - CRM
    return AIRequestCRM, result
}

// Функция для получения данных CRM авторизованного пользователя
func getCRMDataForAI(ctx context.Context, userID string, requestType AIRequestType, params map[string]interface{}) (string, error) {
    var sb strings.Builder
    
    switch requestType {
    case AIRequestAnalytics:
        // Общая статистика
        var totalCustomers, totalDeals int
        var totalValue float64
        
        err := database.Pool.QueryRow(ctx, `
            SELECT 
                (SELECT COUNT(*) FROM crm_customers WHERE user_id = $1::uuid),
                (SELECT COUNT(*) FROM crm_deals WHERE user_id = $1::uuid),
                (SELECT COALESCE(SUM(value), 0) FROM crm_deals WHERE user_id = $1::uuid)
        `, userID).Scan(&totalCustomers, &totalDeals, &totalValue)
        
        if err == nil {
            sb.WriteString(fmt.Sprintf("Всего клиентов: %d\n", totalCustomers))
            sb.WriteString(fmt.Sprintf("Всего сделок: %d\n", totalDeals))
            sb.WriteString(fmt.Sprintf("Общая сумма сделок: %.2f\n", totalValue))
            
            if totalDeals > 0 {
                avgValue := totalValue / float64(totalDeals)
                sb.WriteString(fmt.Sprintf("Средний чек: %.2f\n", avgValue))
            }
        }
        
        // Статистика по этапам
        rows, err := database.Pool.Query(ctx, `
            SELECT stage, COUNT(*), COALESCE(SUM(value), 0)
            FROM crm_deals 
            WHERE user_id = $1::uuid 
            GROUP BY stage
        `, userID)
        if err == nil {
            defer rows.Close()
            sb.WriteString("\nВоронка продаж:\n")
            for rows.Next() {
                var stage string
                var count int
                var sum float64
                if err := rows.Scan(&stage, &count, &sum); err == nil {
                    sb.WriteString(fmt.Sprintf("- %s: %d сделок на сумму %.2f\n", stage, count, sum))
                }
            }
        }
        
    case AIRequestDeal:
        // Данные по сделкам
        if dealID, ok := params["deal_id"]; ok && dealID != "" {
            // Конкретная сделка
            var title, stage, customerName string
            var value float64
            var probability int
            var updatedAt time.Time
            
            err := database.Pool.QueryRow(ctx, `
                SELECT d.title, d.stage, d.value, d.probability, d.updated_at, c.name
                FROM crm_deals d
                JOIN crm_customers c ON c.id = d.customer_id
                WHERE d.id = $1::uuid AND d.user_id = $2::uuid
            `, dealID, userID).Scan(&title, &stage, &value, &probability, &updatedAt, &customerName)
            
            if err == nil {
                sb.WriteString(fmt.Sprintf("Сделка: %s\n", title))
                sb.WriteString(fmt.Sprintf("Клиент: %s\n", customerName))
                sb.WriteString(fmt.Sprintf("Сумма: %.2f\n", value))
                sb.WriteString(fmt.Sprintf("Этап: %s\n", stage))
                sb.WriteString(fmt.Sprintf("Вероятность: %d%%\n", probability))
                sb.WriteString(fmt.Sprintf("Последнее обновление: %s\n", updatedAt.Format("02.01.2006")))
                
                daysSinceUpdate := int(time.Since(updatedAt).Hours() / 24)
                if daysSinceUpdate > 7 {
                    sb.WriteString(fmt.Sprintf("⚠️ Сделка не обновлялась %d дней\n", daysSinceUpdate))
                }
            }
        } else {
            // Общая информация по сделкам
            rows, err := database.Pool.Query(ctx, `
                SELECT d.title, d.stage, d.value, c.name, d.updated_at
                FROM crm_deals d
                JOIN crm_customers c ON c.id = d.customer_id
                WHERE d.user_id = $1::uuid
                ORDER BY d.value DESC
                LIMIT 10
            `, userID)
            if err == nil {
                defer rows.Close()
                sb.WriteString("Топ-10 сделок:\n")
                for rows.Next() {
                    var title, stage, customerName string
                    var value float64
                    var updatedAt time.Time
                    if err := rows.Scan(&title, &stage, &value, &customerName, &updatedAt); err == nil {
                        sb.WriteString(fmt.Sprintf("- %s (клиент: %s): %.2f, этап: %s\n", 
                            title, customerName, value, stage))
                    }
                }
            }
        }
        
    default: // CRM
        // Общие данные по клиентам
        rows, err := database.Pool.Query(ctx, `
            SELECT name, email, status, lead_score, city
            FROM crm_customers
            WHERE user_id = $1::uuid
            ORDER BY lead_score DESC
            LIMIT 20
        `, userID)
        if err == nil {
            defer rows.Close()
            sb.WriteString("Последние клиенты:\n")
            for rows.Next() {
                var name, email, status, city string
                var leadScore float64
                if err := rows.Scan(&name, &email, &status, &leadScore, &city); err == nil {
                    sb.WriteString(fmt.Sprintf("- %s (%s): статус %s, lead score %.0f%%, город %s\n", 
                        name, email, status, leadScore*100, city))
                }
            }
        }
    }
    
    return sb.String(), nil
}

// Функция для получения системного промпта авторизованного пользователя
func getSystemPrompt(requestType AIRequestType, crmData string, userID string) string {
    basePrompt := `Ты AI-ассистент CRM системы. Отвечай ТОЛЬКО на русском языке, структурированно и по делу. Не придумывай то, чего нет в данных.`
    
    switch requestType {
    case AIRequestAnalytics:
        return basePrompt + ` 
        
Ты помогаешь с аналитикой CRM. Вот актуальные данные пользователя:
` + crmData + `

📊 **ФОРМАТ ОТВЕТА ДЛЯ АНАЛИТИКИ:**

📈 **КЛЮЧЕВЫЕ ПОКАЗАТЕЛИ:**
• Всего клиентов: [число]
• Всего сделок: [число]
• Общая сумма сделок: [сумма]
• Средний чек: [сумма]

📊 **ВОРОНКА ПРОДАЖ:**
• Лиды: [количество]
• Переговоры: [количество]
• Предложения: [количество]
• Успешно закрыто: [количество]
• Проиграно: [количество]

📉 **ДИНАМИКА:**
• [краткий анализ изменений за последний месяц]
• [основные тренды]

💡 **РЕКОМЕНДАЦИИ:**
• [конкретный совет по улучшению]
• [что можно оптимизировать]
• [на что обратить внимание]`

    case AIRequestDeal:
        return basePrompt + ` 
        
Ты даешь рекомендации по сделкам. Вот данные пользователя:
` + crmData + `

🤝 **ФОРМАТ ОТВЕТА ДЛЯ СДЕЛОК:**

📌 **ИНФОРМАЦИЯ О СДЕЛКЕ:**
• Название: [название]
• Клиент: [имя клиента]
• Сумма: [сумма] ₽
• Этап: [текущий этап]
• Вероятность: [процент]%
• Последнее обновление: [дата]

🎯 **РЕКОМЕНДАЦИИ:**
• [конкретный совет по работе со сделкой]
• [что сделать для повышения вероятности]
• [следующее действие]

⚠️ **РИСКИ:**
• [если есть - описать риски]
• [что может помешать закрытию]`

    default: // CRM
        return basePrompt + ` 
        
Ты отвечаешь по данным пользователя. Вот актуальная информация:
` + crmData + `

📋 **ФОРМАТ ОТВЕТА:**

✅ **ОТВЕТ НА ВОПРОС:**
[краткий и точный ответ]

📊 **ДАННЫЕ ИЗ CRM:**
• [конкретные данные, относящиеся к вопросу]
• [если нужно - несколько пунктов]

💡 **ДОПОЛНИТЕЛЬНО:**
• [полезная информация по теме]
• [связанные данные]`
    }
}

// ========== СУЩЕСТВУЮЩИЕ ФУНКЦИИ ==========

// getStuckDeals возвращает сделки, которые не двигаются более 7 дней
func getStuckDeals(ctx context.Context, userID string) ([]string, error) {
    rows, err := database.Pool.Query(ctx, `
        SELECT d.title, d.stage, d.updated_at, c.name
        FROM crm_deals d
        JOIN crm_customers c ON c.id = d.customer_id
        WHERE d.user_id = $1::uuid
          AND d.stage NOT IN ('closed_won', 'closed_lost')
          AND d.updated_at < NOW() - INTERVAL '7 days'
        ORDER BY d.updated_at
        LIMIT 10
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var recommendations []string
    for rows.Next() {
        var title, stage, customerName string
        var updatedAt time.Time
        if err := rows.Scan(&title, &stage, &updatedAt, &customerName); err != nil {
            continue
        }
        days := int(time.Since(updatedAt).Hours() / 24)
        line := fmt.Sprintf("📌 Сделка \"%s\" (клиент: %s) на стадии \"%s\" не обновлялась %d дней. Рекомендуется связаться с клиентом.",
            title, customerName, stage, days)
        recommendations = append(recommendations, line)
    }
    return recommendations, nil
}

// getInactiveHighValueClients возвращает клиентов с высоким lead_score, но без активности >14 дней
func getInactiveHighValueClients(ctx context.Context, userID string) ([]string, error) {
    rows, err := database.Pool.Query(ctx, `
        SELECT name, email, lead_score, last_seen
        FROM crm_customers
        WHERE user_id = $1::uuid
        `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var recommendations []string
    for rows.Next() {
        var name, email string
        var leadScore float64
        var lastSeen *time.Time
        if err := rows.Scan(&name, &email, &leadScore, &lastSeen); err != nil {
            continue
        }
        if lastSeen != nil {
            days := int(time.Since(*lastSeen).Hours() / 24)
            if days > 14 {
                line := fmt.Sprintf("🔔 Клиент %s (%s) с lead score %.0f%% неактивен %d дней. Рекомендуется напомнить о себе.",
                    name, email, leadScore*100, days)
                recommendations = append(recommendations, line)
            }
        }
    }
    return recommendations, nil
}

// AIAskHandler - основной обработчик AI запросов (ДВУХРЕЖИМНАЯ ВЕРСИЯ)
func AIAskHandler(c *gin.Context) {
    log.Println("=== AIAskHandler START ===")
    
    // Получаем userID из контекста (проверяем авторизацию)
    userID, exists := c.Get("userID")
    
    var systemPrompt string
    var userIDStr string
    
    if !exists {
        // РЕЖИМ 1: Неавторизованный пользователь - режим консультанта
        log.Println("[AI] Неавторизованный пользователь, режим консультанта")
        systemPrompt = consultantPrompt
        userIDStr = "guest"
    } else {
        // РЕЖИМ 2: Авторизованный пользователь - полный доступ к данным
        log.Printf("[AI] Авторизованный пользователь: %v", userID)
        userIDStr = userID.(string)
        
        var req AskRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            log.Printf("ERROR binding JSON: %v", err)
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        
        // Определяем тип запроса
        requestType, params := detectAIRequestType(req.Question, userIDStr, c)
        
        // Получаем данные CRM
        crmData, err := getCRMDataForAI(c.Request.Context(), userIDStr, requestType, params)
        if err != nil {
            log.Printf("ERROR getting CRM data: %v", err)
            crmData = "Данные временно недоступны"
        }
        
        // Получаем системный промпт с данными пользователя
        systemPrompt = getSystemPrompt(requestType, crmData, userIDStr)
    }

    var req AskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Printf("ERROR binding JSON: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем ключ API
    apiKey := os.Getenv("YANDEX_API_KEY")
    if apiKey == "" {
        log.Println("ERROR: YANDEX_API_KEY not set")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "API ключ не настроен"})
        return
    }

    // Формируем запрос к YandexGPT
    folderID := os.Getenv("YANDEX_FOLDER_ID")
    if folderID == "" {
        log.Println("ERROR: YANDEX_FOLDER_ID not set")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Folder ID не настроен"})
        return
    }

    requestBody := YandexGPTRequest{
        ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
        CompletionOptions: struct {
            Stream      bool    `json:"stream"`
            Temperature float64 `json:"temperature"`
            MaxTokens   int     `json:"maxTokens"`
        }{
            Stream:      false,
            Temperature: 0.3,
            MaxTokens:   2000,
        },
        Messages: []struct {
            Role string `json:"role"`
            Text string `json:"text"`
        }{
            {Role: "system", Text: systemPrompt},
            {Role: "user", Text: req.Question},
        },
    }

    jsonBody, err := json.Marshal(requestBody)
    if err != nil {
        log.Printf("ERROR marshaling request: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка формирования запроса"})
        return
    }

    // Отправляем запрос к YandexGPT
    log.Println("Sending request to YandexGPT...")
    
    client := &http.Client{}
    reqYandex, err := http.NewRequest("POST", "https://llm.api.cloud.yandex.net/foundationModels/v1/completion", bytes.NewBuffer(jsonBody))
    if err != nil {
        log.Printf("ERROR creating request: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса к YandexGPT"})
        return
    }
    
    reqYandex.Header.Set("Content-Type", "application/json")
    reqYandex.Header.Set("Authorization", "Api-Key "+apiKey)
    
    resp, err := client.Do(reqYandex)
    if err != nil {
        log.Printf("ERROR calling YandexGPT: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при обращении к YandexGPT"})
        return
    }
    defer resp.Body.Close()
    log.Printf("YandexGPT response status: %d", resp.StatusCode)

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("ERROR reading response: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа от YandexGPT"})
        return
    }

    if resp.StatusCode != http.StatusOK {
        log.Printf("YandexGPT error: %s", string(body))
        c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("YandexGPT вернул ошибку: %s", string(body))})
        return
    }

    var gptResp YandexGPTResponse
    if err := json.Unmarshal(body, &gptResp); err != nil {
        log.Printf("ERROR parsing response: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга ответа от YandexGPT"})
        return
    }

    if len(gptResp.Result.Alternatives) == 0 {
        log.Println("ERROR: empty response from YandexGPT")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Пустой ответ от YandexGPT"})
        return
    }

    answer := gptResp.Result.Alternatives[0].Message.Text

    // Для авторизованных пользователей добавляем рекомендации если запрошены
    if exists && req.Recommend {
        recommendations, err := getStuckDeals(c.Request.Context(), userIDStr)
        if err == nil && len(recommendations) > 0 {
            answer += "\n\n📋 **Активные рекомендации:**\n" + strings.Join(recommendations, "\n")
        }
        
        inactiveRecommendations, err := getInactiveHighValueClients(c.Request.Context(), userIDStr)
        if err == nil && len(inactiveRecommendations) > 0 {
            answer += "\n\n🔔 **Неактивные клиенты:**\n" + strings.Join(inactiveRecommendations, "\n")
        }
    }

    // Добавляем призыв к авторизации для неавторизованных
    if !exists {
        answer += "\n\n---\n💡 **Хотите больше?** Авторизуйтесь или [зарегистрируйтесь](/register), чтобы получить полный доступ к AI-ассистенту с вашими личными данными и рекомендациями по сделкам!"
    }

    log.Println("=== AIAskHandler END ===")
    c.JSON(http.StatusOK, gin.H{
        "answer": answer,
        "type":   "consultant",
        "authorized": exists,
    })
}