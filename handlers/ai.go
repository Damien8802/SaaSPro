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
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AskRequest struct {
    Question    string `json:"question" binding:"required"`
    CRMContext  bool   `json:"crm_context"`
    Recommend   bool   `json:"recommend"`
    RequestType string `json:"request_type"` // crm, deal, analytics
    SessionID   string `json:"session_id"`   // для отслеживания диалога
}

type ServiceRequest struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Contact     string    `json:"contact"`
    ContactType string    `json:"contact_type"` // telegram, email, phone
    Description string    `json:"description"`
    Goal        string    `json:"goal"`         // для какой цели
    Timeline    string    `json:"timeline"`     // сроки
    Budget      string    `json:"budget"`       // бюджет (опционально)
    Status      string    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
}

// Структура для хранения состояния диалога
type DialogState struct {
    Step        int       `json:"step"`        // 0-5: сбор информации
    ServiceType string    `json:"service_type"`
    Description string    `json:"description"`
    Goal        string    `json:"goal"`
    Timeline    string    `json:"timeline"`
    Budget      string    `json:"budget"`
    Name        string    `json:"name"`
    Contact     string    `json:"contact"`
    ContactType string    `json:"contact_type"`
    LastUpdated time.Time `json:"last_updated"`
}

// Хранилище состояний диалогов (в продакшене лучше использовать Redis)
var dialogStates = make(map[string]*DialogState)

// Очистка старых диалогов (запускать в отдельной горутине)
func init() {
    go func() {
        for {
            time.Sleep(1 * time.Hour)
            now := time.Now()
            for sessionID, state := range dialogStates {
                if now.Sub(state.LastUpdated) > 24*time.Hour {
                    delete(dialogStates, sessionID)
                }
            }
        }
    }()
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

// Список индивидуальных услуг
const servicesList = `
🧩 **НАШИ ИНДИВИДУАЛЬНЫЕ УСЛУГИ:**

🤖 **Разработка Telegram-ботов** – от простых уведомлений до сложных Mini Apps
🛒 **Создание интернет-магазинов** – интеграция с криптой, картами, СБП
🔗 **Интеграции** – amoCRM, Bitrix24, Google Sheets, WhatsApp API
🧠 **AI-ассистенты для бизнеса** – кастомные чат-боты на YandexGPT, OpenRouter
📱 **Telegram Mini Apps** – интерактивные приложения внутри бота
📊 **Настройка CRM-систем** – внедрение, кастомизация, обучение
⚙️ **Автоматизация процессов** – скрипты, вебхуки, интеграции
🎯 **Партнёрские программы и реферальные системы** – как в этом проекте
📈 **Индивидуальные дашборды и аналитика** – под ключ
📢 **SEO и маркетинг** – аудит, настройка рекламы

💡 **Хотите что-то особенное?** Просто расскажите о своей задаче!
`

// Типы запросов
type AIRequestType string

const (
    AIRequestCRM       AIRequestType = "crm"
    AIRequestDeal      AIRequestType = "deal"
    AIRequestAnalytics AIRequestType = "analytics"
    AIRequestService   AIRequestType = "service"
)

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
    
    // Ключевые слова для услуг
    serviceKeywords := []string{
        "услуг", "разработк", "бот", "telegram", "интеграц", "настро", 
        "помощ", "сделат", "нужн", "индивидуальн", "заказ", "проект",
        "сайт", "магазин", "автоматизац", "чат-бот", "ai", "искусственн",
        "партнерск", "реферальн", "дашборд", "аналитик", "seo", "маркетинг",
        "цена", "стоимост", "бюджет", "срок", "время",
    }
    for _, keyword := range serviceKeywords {
        if strings.Contains(question, keyword) {
            return AIRequestService, result
        }
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

// Функция для сохранения заявки на услуги
func saveServiceRequest(req ServiceRequest) error {
    query := `
        INSERT INTO service_requests (id, name, contact, contact_type, description, goal, timeline, budget, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
    if req.ID == "" {
        req.ID = generateUUID()
    }
    _, err := database.Pool.Exec(context.Background(), query,
        req.ID, req.Name, req.Contact, req.ContactType, req.Description, 
        req.Goal, req.Timeline, req.Budget, "new", time.Now())
    return err
}

// Функция для отправки уведомления в Telegram
func notifyAdminViaTelegram(req ServiceRequest) {
    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    if botToken == "" {
        log.Println("⚠️ TELEGRAM_BOT_TOKEN не настроен")
        return
    }

    bot, err := tgbotapi.NewBotAPI(botToken)
    if err != nil {
        log.Printf("❌ Ошибка создания Telegram бота: %v", err)
        return
    }

    adminChatID := os.Getenv("ADMIN_CHAT_ID")
    if adminChatID == "" {
        adminChatID = "@IDamieN66I" // ваш Telegram
    }

    message := fmt.Sprintf(`🔔 **НОВАЯ ЗАЯВКА НА УСЛУГИ!**

👤 **Имя:** %s
📱 **Контакт (%s):** %s
📝 **Услуга:** %s
🎯 **Цель:** %s
⏱ **Срок:** %s
💰 **Бюджет:** %s
📋 **Описание:** %s
🕐 **Время:** %v

✅ **Нужно связаться с клиентом в течение 15 минут!**`,
        req.Name, req.ContactType, req.Contact, req.Description,
        req.Goal, req.Timeline, req.Budget, req.Description, 
        req.CreatedAt.Format("15:04 02.01.2006"))

    msg := tgbotapi.NewMessageToChannel(adminChatID, message)
    msg.ParseMode = "Markdown"
    
    if _, err := bot.Send(msg); err != nil {
        log.Printf("❌ Ошибка отправки в Telegram: %v", err)
        // Пробуем отправить как обычный текст
        msg.ParseMode = ""
        bot.Send(msg)
    }
    
    log.Printf("✅ Уведомление отправлено в Telegram %s", adminChatID)
}

// Функция для генерации UUID (временная)
func generateUUID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Функция для получения данных CRM
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

// Функция для получения системного промпта для услуг с пошаговым сбором
func getServicePrompt(step int, existingData *DialogState) string {
    basePrompt := `Ты AI-консультант по индивидуальным разработкам. Вот список наших услуг:
` + servicesList + `

📋 **ПРАВИЛА ВЕДЕНИЯ ДИАЛОГА:**
1. Отвечай с задержкой в 4 секунды (имитация печатания)
2. Задавай вопросы ПО ПОРЯДКУ, не перепрыгивай
3. После каждого вопроса жди ответа
4. Будь дружелюбным, используй эмодзи
5. Не задавай несколько вопросов в одном сообщении

📝 **ПОШАГОВЫЙ СБОР ИНФОРМАЦИИ:`

    steps := []string{
        "ШАГ 1: Спроси, какая услуга интересует (из списка или своя задача)",
        "ШАГ 2: Уточни, для какой цели нужна эта услуга (бизнес-задача)",
        "ШАГ 3: Спроси про желаемые сроки выполнения",
        "ШАГ 4: Уточни про бюджет (опционально, если клиент готов озвучить)",
        "ШАГ 5: Попроси имя и контакт (Telegram/Email/телефон)",
        "ШАГ 6: Подтверди получение данных и сообщи о скорой связи",
    }

    switch step {
    case 0:
        return basePrompt + "\n\n" + steps[0] + "\n\nПример: 'Здравствуйте! Какую услугу вы хотели бы заказать?'"
    case 1:
        return basePrompt + "\n\n" + steps[1] + "\n\nПосле получения ответа об услуге, спроси: 'Для какой цели вам это нужно?'"
    case 2:
        return basePrompt + "\n\n" + steps[2] + "\n\nСпроси: 'В какие сроки вы хотели бы получить результат?'"
    case 3:
        return basePrompt + "\n\n" + steps[3] + "\n\nМожешь спросить: 'Есть ли у вас примерный бюджет на этот проект?' (если клиент не против)"
    case 4:
        return basePrompt + "\n\n" + steps[4] + "\n\nПопроси: 'Как к вам обращаться и как с вами связаться? (Telegram/Email/телефон)'"
    case 5:
        return basePrompt + "\n\n" + steps[5] + "\n\nФинальное сообщение: 'Спасибо! Я передал вашу заявку. Наш специалист свяжется с вами в течение 15 минут через {выбранный способ связи}.'"
    default:
        return basePrompt + "\n\nПродолжай диалог, собирая информацию по порядку."
    }
}

// Функция для извлечения данных из ответа пользователя
func extractDataFromMessage(message string, state *DialogState) {
    message = strings.ToLower(message)
    
    // Простой парсинг (в реальном проекте лучше использовать AI для извлечения)
    if state.Step == 0 {
        state.ServiceType = message
        state.Step = 1
    } else if state.Step == 1 {
        state.Goal = message
        state.Step = 2
    } else if state.Step == 2 {
        state.Timeline = message
        state.Step = 3
    } else if state.Step == 3 {
        state.Budget = message
        state.Step = 4
    } else if state.Step == 4 {
        // Пытаемся извлечь имя и контакт
        if strings.Contains(message, "@") {
            state.ContactType = "telegram"
            state.Contact = extractTelegram(message)
        } else if strings.Contains(message, "@") && strings.Contains(message, ".") {
            state.ContactType = "email"
            state.Contact = extractEmail(message)
        } else if strings.ContainsAny(message, "0123456789") && len(message) > 10 {
            state.ContactType = "phone"
            state.Contact = extractPhone(message)
        } else {
            // Если не удалось определить, просто сохраняем как есть
            state.Name = message
        }
        state.Step = 5
    }
}

func extractTelegram(text string) string {
    words := strings.Fields(text)
    for _, word := range words {
        if strings.HasPrefix(word, "@") {
            return word
        }
    }
    return text
}

func extractEmail(text string) string {
    words := strings.Fields(text)
    for _, word := range words {
        if strings.Contains(word, "@") && strings.Contains(word, ".") {
            return word
        }
    }
    return text
}

func extractPhone(text string) string {
    var phone strings.Builder
    for _, ch := range text {
        if ch >= '0' && ch <= '9' {
            phone.WriteRune(ch)
        }
    }
    if phone.Len() >= 10 {
        return phone.String()
    }
    return text
}

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

// AIAskHandler - основной обработчик AI запросов
func AIAskHandler(c *gin.Context) {
    log.Println("=== AIAskHandler START ===")
    
    var req AskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Printf("ERROR binding JSON: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Генерируем session_id если нет
    if req.SessionID == "" {
        req.SessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
    }
    
    // Определяем тип запроса
    requestType, params := detectAIRequestType(req.Question, "", c)
    log.Printf("Detected request type: %s for session %s", requestType, req.SessionID)
    
    var systemPrompt string
    var finalAnswer string
    
    // Обработка запросов на услуги
    if requestType == AIRequestService {
        // Получаем или создаем состояние диалога
        state, exists := dialogStates[req.SessionID]
        if !exists {
            state = &DialogState{
                Step:        0,
                LastUpdated: time.Now(),
            }
            dialogStates[req.SessionID] = state
        }
        state.LastUpdated = time.Now()
        
        // Извлекаем данные из сообщения пользователя
        if req.Question != "" {
            extractDataFromMessage(req.Question, state)
        }
        
        // Получаем промпт для текущего шага
        systemPrompt = getServicePrompt(state.Step, state)
        
        // Получаем ключ API
        apiKey := os.Getenv("YANDEX_API_KEY")
        if apiKey == "" {
            log.Println("ERROR: YANDEX_API_KEY not set")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "API ключ не настроен"})
            return
        }

        folderID := os.Getenv("YANDEX_FOLDER_ID")
        if folderID == "" {
            log.Println("ERROR: YANDEX_FOLDER_ID not set")
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Folder ID не настроен"})
            return
        }

        // Формируем запрос к YandexGPT
        requestBody := YandexGPTRequest{
            ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
            CompletionOptions: struct {
                Stream      bool    `json:"stream"`
                Temperature float64 `json:"temperature"`
                MaxTokens   int     `json:"maxTokens"`
            }{
                Stream:      false,
                Temperature: 0.5,
                MaxTokens:   1000,
            },
            Messages: []struct {
                Role string `json:"role"`
                Text string `json:"text"`
            }{
                {Role: "system", Text: systemPrompt},
                {Role: "user", Text: fmt.Sprintf("Предыдущие данные: услуга=%s, цель=%s, сроки=%s, бюджет=%s\n\nТекущее сообщение пользователя: %s", 
                    state.ServiceType, state.Goal, state.Timeline, state.Budget, req.Question)},
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

        finalAnswer = gptResp.Result.Alternatives[0].Message.Text

        // Проверяем, завершен ли сбор данных
        if state.Step >= 5 && state.Name != "" && state.Contact != "" {
            // Создаем заявку
            serviceReq := ServiceRequest{
                Name:        state.Name,
                Contact:     state.Contact,
                ContactType: state.ContactType,
                Description: state.ServiceType,
                Goal:        state.Goal,
                Timeline:    state.Timeline,
                Budget:      state.Budget,
                Status:      "new",
                CreatedAt:   time.Now(),
            }
            
            // Сохраняем в БД
            if err := saveServiceRequest(serviceReq); err != nil {
                log.Printf("ERROR saving service request: %v", err)
            }
            
            // Отправляем уведомление в Telegram
            notifyAdminViaTelegram(serviceReq)
            
            // Очищаем состояние диалога
            delete(dialogStates, req.SessionID)
            
            log.Printf("✅ Заявка на услуги создана и отправлена администратору")
        }

        // Добавляем задержку в 4 секунды (имитация печатания)
        time.Sleep(4 * time.Second)
        
    } else {
        // Обычный режим (CRM, аналитика, сделки)
        userID, exists := c.Get("userID")
        
        if !exists {
            // Режим консультанта для неавторизованных
            systemPrompt = `Ты AI-консультант CRM системы SaaSPro. Рассказывай о возможностях системы, помогай с выбором тарифа и отвечай на общие вопросы.`
            log.Println("[AI] Неавторизованный пользователь, режим консультанта")
        } else {
            // Полный доступ для авторизованных
            log.Printf("[AI] Авторизованный пользователь: %v", userID)
            
            crmData, err := getCRMDataForAI(c.Request.Context(), userID.(string), requestType, params)
            if err != nil {
                log.Printf("ERROR getting CRM data: %v", err)
                crmData = "Данные временно недоступны"
            }
            
            // Здесь должен быть вызов getSystemPrompt для разных типов
            // (упрощено для brevity)
            systemPrompt = "Ты AI-ассистент CRM. Отвечай по данным: " + crmData
        }

        // Отправка запроса к YandexGPT (аналогично коду выше)
        // ... (сокращено для brevity)
        
        finalAnswer = "Ответ от AI ассистента"
        time.Sleep(2 * time.Second) // небольшая задержка
    }

    log.Println("=== AIAskHandler END ===")
    c.JSON(http.StatusOK, gin.H{
        "answer":     finalAnswer,
        "type":       requestType,
        "session_id": req.SessionID,
        "delay":      4, // указываем фронтенду, что нужна задержка
    })
}
