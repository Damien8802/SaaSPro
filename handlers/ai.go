package handlers

import (
    "bytes"
    "context"
    "crypto/tls"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/smtp"
    "os"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "subscription-system/database"
)

type AskRequest struct {
    Question    string  `json:"question" binding:"required"`
    CRMContext  bool    `json:"crm_context"`
    Recommend   bool    `json:"recommend"`
    RequestType string  `json:"request_type"`
    SessionID   string  `json:"session_id"`
    Model       string  `json:"model"`
    Temperature float64 `json:"temperature"`
}

type PriceInfo struct {
    ServiceName  string
    DisplayPrice float64
    SourceCount  int
    LastUpdated  time.Time
    Sources      []string
}

type SearchResult struct {
    Title       string
    Description string
    Price       float64
    Source      string
}

type DialogState struct {
    UserName          string             `json:"user_name"`
    UserService       string             `json:"user_service"`
    UserServiceName   string             `json:"user_service_name"`
    BasePrice         float64            `json:"base_price"`
    CalculatedPrice   float64            `json:"calculated_price"`
    UserPhone         string             `json:"user_phone"`
    UserEmail         string             `json:"user_email"`
    UserMessenger     string             `json:"user_messenger"`
    Messages          []string           `json:"messages"`
    LastUpdated       time.Time
    GreetingShown     bool
    AwaitingPhone     bool
    AwaitingEmail     bool
    AwaitingMessenger bool
    Completed         bool
    DesignAsked       bool
    DesignAnswer      string
    TechHelpAsked     bool
    TechHelpAdded     bool
    DeadlineAsked     bool
    Deadline          string
    AdditionalService string
    AdditionalPrice   float64
    CurrentStep       int
    LastSearchResults []SearchResult
    AwaitingData      string
    TempData          map[string]string
    CurrentOrder      map[string]interface{}
}

type SearchCache struct {
    Results   []SearchResult
    ExpiresAt time.Time
    Query     string
}

type TelegramNotify struct {
    ChatID    string `json:"chat_id"`
    Text      string `json:"text"`
    ParseMode string `json:"parse_mode"`
}

var (
    dialogStates = make(map[string]*DialogState)
    dialogMutex  = &sync.RWMutex{}
    searchCache  = make(map[string]*SearchCache)
    cacheMutex   = &sync.RWMutex{}
    cacheTTL     = 1 * time.Hour

    telegramBotToken string
    telegramChatID   string
    yandexApiKey     string
    yandexFolderID   string

    emailTo       string
    emailFrom     string
    emailPassword string
    smtpHost      string
    smtpPort      string
)

// УСЛУГИ И ЦЕНЫ
var servicesList = map[string]map[string]interface{}{
    "crm":         {"name": "CRM система", "price": 2990, "period": "мес", "url": "/crm", "description": "Управление клиентами, сделки, воронка продаж"},
    "vpn":         {"name": "VPN сервис", "price": 299, "period": "мес", "url": "/vpn", "description": "Stealth VPN, WireGuard, высокая скорость"},
    "ai":          {"name": "AI ассистент", "price": 9900, "period": "мес", "url": "/ai", "description": "YandexGPT, голосовой ввод, загрузка файлов"},
    "marketplace": {"name": "Маркетплейс", "price": 6900, "period": "мес", "url": "/marketplace", "description": "Ozon, Wildberries, управление заказами"},
    "1c":          {"name": "1С интеграция", "price": 4900, "period": "мес", "url": "/integration/1c", "description": "Синхронизация с 1С:Бухгалтерия"},
    "telegram":    {"name": "Telegram бот", "price": 2900, "period": "мес", "url": "/telegram-bot", "description": "Уведомления, чат-бот, рассылки"},
    "whatsapp":    {"name": "WhatsApp", "price": 2900, "period": "мес", "url": "/whatsapp", "description": "WhatsApp Business API"},
    "bitrix":      {"name": "Bitrix24", "price": 4900, "period": "мес", "url": "/integration/bitrix", "description": "Синхронизация с Bitrix24"},
    "shop":        {"name": "Интернет-магазин", "price": 150000, "period": "разово", "url": "/shop", "description": "Магазин под ключ"},
    "seo":         {"name": "SEO продвижение", "price": 30000, "period": "мес", "url": "/seo", "description": "Продвижение в топ"},
    "hr":          {"name": "HR модуль", "price": 5000, "period": "мес", "url": "/hr", "description": "Управление персоналом"},
    "finance":     {"name": "Финансы", "price": 4000, "period": "мес", "url": "/finance", "description": "Учет и отчетность"},
    "logistics":   {"name": "Логистика", "price": 3000, "period": "мес", "url": "/logistics", "description": "Управление доставкой"},
}

func init() {
    godotenv.Load()
    telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
    telegramChatID = os.Getenv("ADMIN_CHAT_ID")
    yandexApiKey = os.Getenv("YANDEX_SEARCH_API_KEY")
    yandexFolderID = os.Getenv("YANDEX_FOLDER_ID")

    emailTo = os.Getenv("EMAIL_TO")
    if emailTo == "" {
        emailTo = "Skorpion_88-88@mail.ru"
    }
    emailFrom = os.Getenv("EMAIL_FROM")
    emailPassword = os.Getenv("EMAIL_PASSWORD")
    smtpHost = os.Getenv("SMTP_HOST")
    if smtpHost == "" {
        smtpHost = "smtp.mail.ru"
    }
    smtpPort = os.Getenv("SMTP_PORT")
    if smtpPort == "" {
        smtpPort = "587"
    }

    log.Println("==================================================")
    log.Printf("🔍 Яндекс Поиск: %s", maskString(yandexApiKey, 10))
    log.Printf("📧 Email: %s", emailTo)
    log.Printf("📦 Доступно услуг: %d", len(servicesList))
    log.Println("✅ AI ассистент ПОЛНОСТЬЮ инициализирован")
    log.Println("==================================================")

    os.MkdirAll("orders", 0755)
    go startCleanupRoutine()
    go startCacheCleanupRoutine()
}

func maskString(s string, visible int) string {
    if len(s) <= visible {
        return s
    }
    return s[:visible] + "..." + s[len(s)-3:]
}

func startCleanupRoutine() {
    for {
        time.Sleep(1 * time.Hour)
        dialogMutex.Lock()
        now := time.Now()
        for id, state := range dialogStates {
            if now.Sub(state.LastUpdated) > 24*time.Hour {
                delete(dialogStates, id)
            }
        }
        dialogMutex.Unlock()
    }
}

func startCacheCleanupRoutine() {
    for {
        time.Sleep(30 * time.Minute)
        cacheMutex.Lock()
        now := time.Now()
        for key, cache := range searchCache {
            if now.After(cache.ExpiresAt) {
                delete(searchCache, key)
            }
        }
        cacheMutex.Unlock()
    }
}

func getCacheKey(query string) string {
    return strings.ToLower(strings.TrimSpace(query))
}

// ПОИСК В ИНТЕРНЕТЕ
func searchInternet(query string) ([]SearchResult, error) {
    cacheKey := getCacheKey(query)
    cacheMutex.RLock()
    if cached, exists := searchCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
        cacheMutex.RUnlock()
        return cached.Results, nil
    }
    cacheMutex.RUnlock()

    if yandexApiKey == "" || yandexFolderID == "" {
        return nil, fmt.Errorf("поиск не настроен")
    }

    log.Printf("🔍 Поиск: %s", query)

    searchQueries := []string{
        query,
        fmt.Sprintf("%s цена", query),
        fmt.Sprintf("%s сколько стоит", query),
        fmt.Sprintf("%s дизайн", query),
        fmt.Sprintf("%s советы", query),
    }

    var allResults []SearchResult

    for _, sq := range searchQueries {
        cleanQuery := strings.ReplaceAll(sq, "–", "-")
        cleanQuery = strings.ReplaceAll(cleanQuery, "—", "-")
        if len(cleanQuery) > 100 {
            cleanQuery = cleanQuery[:100]
        }

        requestBody := map[string]interface{}{
            "query": map[string]interface{}{
                "query_text":  cleanQuery,
                "search_type": "SEARCH_TYPE_RU",
            },
            "max_docs": 10,
        }

        jsonBody, _ := json.Marshal(requestBody)
        url := fmt.Sprintf("https://searchapi.api.cloud.yandex.net/v2/web/search?folderId=%s", yandexFolderID)

        req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Api-Key "+yandexApiKey)

        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            continue
        }

        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            continue
        }

        var result struct {
            RawData string `json:"rawData"`
        }
        if err := json.Unmarshal(body, &result); err != nil {
            continue
        }

        xmlData, _ := base64.StdEncoding.DecodeString(result.RawData)
        xmlStr := string(xmlData)

        titleRegex := regexp.MustCompile(`<title>(.*?)</title>`)
        passageRegex := regexp.MustCompile(`<passage>(.*?)</passage>`)
        priceRegex := regexp.MustCompile(`(\d{1,3}(?:[.\s]?\d{3})*)\s*(?:тыс\.?|тысяч|₽|руб)`)

        titles := titleRegex.FindAllStringSubmatch(xmlStr, -1)
        passages := passageRegex.FindAllStringSubmatch(xmlStr, -1)

        for _, title := range titles {
            if len(title) > 1 {
                res := SearchResult{Title: title[1], Description: "", Source: sq}
                matches := priceRegex.FindAllStringSubmatch(title[1], -1)
                for _, match := range matches {
                    if len(match) > 1 {
                        priceStr := strings.ReplaceAll(match[1], " ", "")
                        price, _ := strconv.ParseFloat(priceStr, 64)
                        if strings.Contains(match[0], "тыс") {
                            price *= 1000
                        }
                        if price >= 5000 && price <= 5000000 {
                            res.Price = price
                            break
                        }
                    }
                }
                allResults = append(allResults, res)
            }
        }

        for _, passage := range passages {
            if len(passage) > 1 {
                res := SearchResult{Title: "", Description: passage[1], Source: sq}
                matches := priceRegex.FindAllStringSubmatch(passage[1], -1)
                for _, match := range matches {
                    if len(match) > 1 {
                        priceStr := strings.ReplaceAll(match[1], " ", "")
                        price, _ := strconv.ParseFloat(priceStr, 64)
                        if strings.Contains(match[0], "тыс") {
                            price *= 1000
                        }
                        if price >= 5000 && price <= 5000000 {
                            res.Price = price
                            break
                        }
                    }
                }
                allResults = append(allResults, res)
            }
        }
        time.Sleep(100 * time.Millisecond)
    }

    uniqueResults := make(map[string]SearchResult)
    for _, r := range allResults {
        if r.Title != "" || r.Description != "" {
            key := r.Title
            if key == "" && len(r.Description) > 50 {
                key = r.Description[:50]
            }
            if _, exists := uniqueResults[key]; !exists {
                uniqueResults[key] = r
            }
        }
    }

    results := make([]SearchResult, 0, len(uniqueResults))
    for _, r := range uniqueResults {
        results = append(results, r)
    }

    cacheMutex.Lock()
    searchCache[cacheKey] = &SearchCache{Results: results, ExpiresAt: time.Now().Add(cacheTTL), Query: query}
    cacheMutex.Unlock()

    log.Printf("✅ Найдено %d результатов", len(results))
    return results, nil
}

func getAveragePrice(query string) float64 {
    results, err := searchInternet(query + " цена")
    if err != nil {
        return 50000
    }
    var prices []float64
    for _, r := range results {
        if r.Price > 0 {
            prices = append(prices, r.Price)
        }
    }
    if len(prices) > 0 {
        sort.Float64s(prices)
        return prices[len(prices)/2]
    }
    return 50000
}

func getAdvice(query string) string {
    results, err := searchInternet(query + " советы")
    if err != nil {
        return getDefaultAdvice(query)
    }
    var advice strings.Builder
    advice.WriteString("💡 **Рекомендации:**\n\n")
    count := 0
    for _, r := range results {
        if count >= 3 {
            break
        }
        if r.Description != "" {
            advice.WriteString(fmt.Sprintf("• %s\n\n", truncateText(r.Description, 200)))
            count++
        } else if r.Title != "" {
            advice.WriteString(fmt.Sprintf("• %s\n\n", r.Title))
            count++
        }
    }
    if count == 0 {
        return getDefaultAdvice(query)
    }
    return advice.String()
}

func getDefaultAdvice(query string) string {
    advice := "💡 **Рекомендации:**\n\n"
    if strings.Contains(strings.ToLower(query), "дизайн") {
        advice += "• Используйте современный минимализм\n• Адаптивный дизайн обязателен\n• Используйте контрастные цвета\n"
    } else if strings.Contains(strings.ToLower(query), "telegram") {
        advice += "• Добавьте inline-кнопки\n• Используйте WebApp\n• Настройте платежи через Telegram Stars\n"
    } else {
        advice += "• Изучите конкурентов\n• Сделайте акцент на UX\n• Добавьте аналитику\n"
    }
    return advice
}

func truncateText(text string, maxLen int) string {
    if len(text) <= maxLen {
        return text
    }
    return text[:maxLen] + "..."
}

func formatPrice(price float64) string {
    if price >= 1000000 {
        return fmt.Sprintf("%.1f млн", price/1000000)
    }
    if price >= 1000 {
        return fmt.Sprintf("%.0f тыс", price/1000)
    }
    return fmt.Sprintf("%.0f", price)
}

func getGreeting() string {
    hour := time.Now().Hour()
    switch {
    case hour >= 5 && hour < 12:
        return "Доброе утро"
    case hour >= 12 && hour < 17:
        return "Добрый день"
    case hour >= 17 && hour < 24:
        return "Добрый вечер"
    default:
        return "Доброй ночи"
    }
}

func isNameResponse(query string) bool {
    q := strings.ToLower(query)
    if len(q) >= 2 && len(q) <= 15 {
        match, _ := regexp.MatchString(`^[а-яa-z]+$`, q)
        if match {
            notName := []string{"бот", "телеграм", "crm", "vpn", "помощь", "привет", "хочу", "купить", "заказать", "добавь", "создай", "покажи", "сколько", "цена"}
            for _, w := range notName {
                if strings.Contains(q, w) {
                    return false
                }
            }
            return true
        }
    }
    return false
}

func detectMessenger(text string) string {
    lower := strings.ToLower(text)
    if strings.Contains(lower, "telegram") || strings.Contains(lower, "тг") || strings.Contains(lower, "телеграм") {
        return "Telegram"
    }
    if strings.Contains(lower, "whatsapp") || strings.Contains(lower, "ватсап") {
        return "WhatsApp"
    }
    return ""
}

func saveToFile(userName, service, phone, messenger, price, deadline, design, techHelp, additionalInfo string) {
    os.MkdirAll("orders", 0755)
    filename := fmt.Sprintf("orders/order_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
    content := fmt.Sprintf("╔════════════════════════════════════════╗\n")
    content += fmt.Sprintf("║         🔥 НОВАЯ ЗАЯВКА 🔥            ║\n")
    content += fmt.Sprintf("╚════════════════════════════════════════╝\n\n")
    content += fmt.Sprintf("📅 Дата: %s\n", time.Now().Format("2006-01-02 15:04:05"))
    content += fmt.Sprintf("👤 Клиент: %s\n", userName)
    content += fmt.Sprintf("📋 Услуга: %s\n", service)
    content += fmt.Sprintf("💰 Стоимость: %s ₽\n", price)
    if phone != "" {
        content += fmt.Sprintf("📱 Телефон: %s\n", phone)
    }
    if messenger != "" {
        content += fmt.Sprintf("💬 Мессенджер: %s\n", messenger)
    }
    os.WriteFile(filename, []byte(content), 0644)
    log.Printf("✅ Заявка сохранена: %s", filename)
}

func sendEmailNotification(userName, service, phone, messenger, price, deadline, design, techHelp, additionalInfo string) {
    if emailFrom == "" || emailPassword == "" {
        return
    }
    subject := fmt.Sprintf("🔥 Новая заявка от %s", userName)
    body := fmt.Sprintf("<h2>НОВАЯ ЗАЯВКА</h2><p>Клиент: %s</p><p>Услуга: %s</p><p>Цена: %s</p>", userName, service, price)
    
    auth := smtp.PlainAuth("", emailFrom, emailPassword, smtpHost)
    addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
    msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html\r\n\r\n%s", emailTo, subject, body))
    
    conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: smtpHost})
    if err != nil {
        return
    }
    defer conn.Close()
    
    client, err := smtp.NewClient(conn, smtpHost)
    if err != nil {
        return
    }
    defer client.Quit()
    
    client.Auth(auth)
    client.Mail(emailFrom)
    client.Rcpt(emailTo)
    w, _ := client.Data()
    w.Write(msg)
    w.Close()
    log.Printf("✅ Email отправлен")
}

// ГЛАВНЫЙ ОБРАБОТЧИК
func AIAskHandler(c *gin.Context) {
    var req AskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.SessionID == "" {
        req.SessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
    }

    dialogMutex.Lock()
    state, exists := dialogStates[req.SessionID]
    if !exists {
        state = &DialogState{
            Messages:    []string{},
            LastUpdated: time.Now(),
            TempData:    make(map[string]string),
            CurrentOrder: make(map[string]interface{}),
        }
        dialogStates[req.SessionID] = state
    }
    state.LastUpdated = time.Now()
    dialogMutex.Unlock()

    question := strings.TrimSpace(req.Question)
    lowerQ := strings.ToLower(question)

    // Сохраняем в историю
    dialogMutex.Lock()
    state.Messages = append(state.Messages, "user: "+question)
    if len(state.Messages) > 50 {
        state.Messages = state.Messages[len(state.Messages)-50:]
    }
    dialogMutex.Unlock()

    answer := ""
    phoneRegex := regexp.MustCompile(`^(\+7|8|7)?[\s-]?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}$`)

    // ОЖИДАНИЕ ТЕЛЕФОНА
    if state.AwaitingPhone {
        if phoneRegex.MatchString(question) {
            state.UserPhone = question
            state.AwaitingPhone = false
            state.AwaitingMessenger = true
            answer = "📱 Отлично! На этом номере есть мессенджер? (Telegram/WhatsApp)"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "📱 Пожалуйста, введите номер телефона в формате: +7 XXX XXX-XX-XX"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    // ОЖИДАНИЕ МЕССЕНДЖЕРА
    if state.AwaitingMessenger {
        messenger := detectMessenger(question)
        if messenger != "" {
            state.UserMessenger = messenger
            state.AwaitingMessenger = false
            state.Completed = true
            
            go saveToFile(state.UserName, state.UserService, state.UserPhone, state.UserMessenger,
                formatPrice(state.CalculatedPrice), state.Deadline, state.DesignAnswer, 
                map[bool]string{true: "да", false: "нет"}[state.TechHelpAdded], state.AdditionalService)
            go sendEmailNotification(state.UserName, state.UserService, state.UserPhone, state.UserMessenger,
                formatPrice(state.CalculatedPrice), state.Deadline, state.DesignAnswer, 
                map[bool]string{true: "да", false: "нет"}[state.TechHelpAdded], state.AdditionalService)

            answer = "✅ **Ваша заявка принята!**\n\n👨‍💻 Специалист свяжется с вами через 15 минут.\n\n🌟 Всего наилучшего!"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "Пожалуйста, укажите мессенджер: Telegram или WhatsApp"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    // ПРИВЕТСТВИЕ
    if !state.GreetingShown {
        greeting := getGreeting()
        answer = fmt.Sprintf(`%s! 👋 Я AI-помощник **SaaSPro**.

🎯 **Как к вам можно обращаться?**

Я умею:
• 📊 Работать с CRM (добавлять клиентов, сделки)
• 🔒 Подключать VPN
• 🛒 Оформлять заказы на услуги
• 👥 Создавать команды в TeamSphere
• 💰 Рассказывать о ценах
• 🔍 Искать информацию в интернете

Просто скажите, что нужно сделать!`, greeting)
        state.GreetingShown = true
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // ПОЛУЧЕНИЕ ИМЕНИ
    if state.UserName == "" {
        if isNameResponse(question) {
            state.UserName = question
            answer = fmt.Sprintf(`Приятно познакомиться, **%s**! 🌟

📝 **Чем могу помочь?**

💰 **Узнать цены** - "сколько стоит CRM"
🛒 **Купить услугу** - "хочу купить VPN"
📊 **CRM** - "добавь клиента ООО Ромашка"
👥 **TeamSphere** - "создай команду Маркетинг"
🔍 **Поиск** - "найди информацию о..."
❓ **Помощь** - "что умеешь"

Что будем делать?`, state.UserName)
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "Пожалуйста, напишите ваше имя:"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    // ПОЛУЧЕНИЕ УСЛУГИ (СБОР ЗАЯВКИ)
    if state.UserService == "" && !strings.Contains(lowerQ, "покажи") && !strings.Contains(lowerQ, "список") &&
        !strings.Contains(lowerQ, "сколько") && !strings.Contains(lowerQ, "цена") && !strings.Contains(lowerQ, "стоимость") &&
        !strings.Contains(lowerQ, "хочу") && !strings.Contains(lowerQ, "купить") && !strings.Contains(lowerQ, "заказать") &&
        !strings.Contains(lowerQ, "помощь") && !strings.Contains(lowerQ, "что умее") && !strings.Contains(lowerQ, "найди") {
        
        price := getAveragePrice(question)
        state.UserService = question
        state.BasePrice = price
        state.CalculatedPrice = price

        advice := getAdvice(question)

        answer = fmt.Sprintf("🎯 **Услуга:** %s\n\n", question)
        answer += fmt.Sprintf("💰 **Примерная стоимость:** %s ₽\n\n", formatPrice(price))
        answer += advice + "\n"
        answer += "🎨 **Расскажите о пожеланиях по дизайну:**\n• Есть готовый дизайн?\n• Нужна разработка с нуля?\n• Или могу предложить варианты"

        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // ВОПРОС О ДИЗАЙНЕ
    if !state.DesignAsked && state.UserService != "" {
        state.DesignAnswer = question
        state.DesignAsked = true

        if strings.Contains(lowerQ, "дизайн") || strings.Contains(lowerQ, "вариант") {
            designAdvice := getAdvice(state.UserService + " дизайн")
            answer = designAdvice + "\n\n💡 **Хотите добавить техническую поддержку?**\n\n🛠️ Техподдержка: 15 000 ₽/мес\n\nДобавляем? (Да/Нет)"
        } else {
            answer = "💡 **Хотите добавить техническую поддержку?**\n\n🛠️ Техподдержка: 15 000 ₽/мес\n\nДобавляем? (Да/Нет)"
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // ТЕХПОДДЕРЖКА
    if !state.TechHelpAsked && state.DesignAsked {
        state.TechHelpAsked = true

        if strings.Contains(lowerQ, "да") {
            state.CalculatedPrice += 15000
            state.TechHelpAdded = true
            answer = fmt.Sprintf("✅ Техподдержка добавлена!\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "⏰ **В какие сроки нужен проект?**\n\n• Чем быстрее, тем лучше\n• 2 недели\n• 1 месяц"
            state.DeadlineAsked = true
        } else if strings.Contains(lowerQ, "нет") {
            answer = fmt.Sprintf("✅ Хорошо!\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "⏰ **В какие сроки нужен проект?**\n\n• Чем быстрее, тем лучше\n• 2 недели\n• 1 месяц"
            state.DeadlineAsked = true
        } else {
            answer = "Пожалуйста, ответьте **Да** или **Нет**: добавить техподдержку?"
            state.TechHelpAsked = false
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // СРОКИ
    if state.DeadlineAsked && !state.AwaitingPhone {
        if strings.Contains(lowerQ, "быстре") || strings.Contains(lowerQ, "сроч") {
            originalPrice := state.CalculatedPrice
            state.CalculatedPrice = state.CalculatedPrice * 1.3
            state.Deadline = "Срочно (5-7 дней)"
            answer = fmt.Sprintf("🚀 **Срочная разработка!**\n\n💰 Стоимость: %s ₽\n(было %s ₽, +30%% за срочность)\n\n❓ Согласны? (Да/Нет)", 
                formatPrice(state.CalculatedPrice), formatPrice(originalPrice))
        } else if strings.Contains(lowerQ, "2 недел") || strings.Contains(lowerQ, "две") {
            state.Deadline = "2 недели"
            answer = fmt.Sprintf("✅ Срок: 2 недели\n\n💰 **Итоговая стоимость:** %s ₽\n\n📝 **Для оформления оставьте номер телефона:**", formatPrice(state.CalculatedPrice))
            state.AwaitingPhone = true
        } else if strings.Contains(lowerQ, "месяц") {
            state.Deadline = "1 месяц"
            answer = fmt.Sprintf("✅ Срок: 1 месяц\n\n💰 **Итоговая стоимость:** %s ₽\n\n📝 **Для оформления оставьте номер телефона:**", formatPrice(state.CalculatedPrice))
            state.AwaitingPhone = true
        } else {
            answer = "Пожалуйста, укажите срок:\n• Чем быстрее, тем лучше\n• 2 недели\n• 1 месяц"
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // СОГЛАСИЕ НА СРОЧНОСТЬ
    if state.Deadline == "Срочно (5-7 дней)" && !state.AwaitingPhone {
        if strings.Contains(lowerQ, "да") {
            answer = "📝 **Для оформления оставьте номер телефона:**"
            state.AwaitingPhone = true
        } else if strings.Contains(lowerQ, "нет") {
            state.DeadlineAsked = false
            state.Deadline = ""
            answer = "Хорошо, выберите другой срок:\n• 2 недели\n• 1 месяц"
        } else {
            answer = "Пожалуйста, ответьте **Да** или **Нет**"
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    // ОБРАБОТКА КОМАНД
    if answer == "" {
        answer = processCommands(lowerQ, state, c.Request.Context())
    }

    c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
}

// ОБРАБОТКА КОМАНД
func processCommands(q string, state *DialogState, ctx context.Context) string {
    userName := state.UserName

    // ЦЕНЫ
    if strings.Contains(q, "сколько стоит") || strings.Contains(q, "цена") || strings.Contains(q, "стоимость") {
        if strings.Contains(q, "crm") {
            s := servicesList["crm"]
            return fmt.Sprintf("📊 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "vpn") || strings.Contains(q, "впн") {
            s := servicesList["vpn"]
            return fmt.Sprintf("🔒 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "ai") || strings.Contains(q, "ассистент") {
            s := servicesList["ai"]
            return fmt.Sprintf("🧠 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "маркетплейс") {
            s := servicesList["marketplace"]
            return fmt.Sprintf("📦 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "1с") || strings.Contains(q, "1c") {
            s := servicesList["1c"]
            return fmt.Sprintf("🔌 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "telegram") || strings.Contains(q, "телеграм") {
            s := servicesList["telegram"]
            return fmt.Sprintf("📱 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "whatsapp") {
            s := servicesList["whatsapp"]
            return fmt.Sprintf("💬 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "bitrix") {
            s := servicesList["bitrix"]
            return fmt.Sprintf("🔄 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "магазин") || strings.Contains(q, "интернет") {
            s := servicesList["shop"]
            return fmt.Sprintf("🛒 **%s** - %d ₽ (%s)\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "seo") {
            s := servicesList["seo"]
            return fmt.Sprintf("📈 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "hr") {
            s := servicesList["hr"]
            return fmt.Sprintf("👥 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "финанс") {
            s := servicesList["finance"]
            return fmt.Sprintf("💰 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }
        if strings.Contains(q, "логистик") {
            s := servicesList["logistics"]
            return fmt.Sprintf("🚚 **%s** - %d ₽/%s\n%s\n📍 %s", s["name"], s["price"], s["period"], s["description"], s["url"])
        }

        var result strings.Builder
        result.WriteString("💰 **Наши услуги и цены:**\n\n")
        for _, s := range servicesList {
            result.WriteString(fmt.Sprintf("• %s - %d ₽/%s\n", s["name"], s["price"], s["period"]))
        }
        result.WriteString("\nНапишите **'хочу [услуга]'** для заказа!")
        return result.String()
    }

    // КУПИТЬ
    if strings.Contains(q, "хочу") || strings.Contains(q, "купить") || strings.Contains(q, "заказать") {
        var selectedKey string
        if strings.Contains(q, "crm") {
            selectedKey = "crm"
        } else if strings.Contains(q, "vpn") || strings.Contains(q, "впн") {
            selectedKey = "vpn"
        } else if strings.Contains(q, "ai") || strings.Contains(q, "ассистент") {
            selectedKey = "ai"
        } else if strings.Contains(q, "маркетплейс") {
            selectedKey = "marketplace"
        } else if strings.Contains(q, "1с") || strings.Contains(q, "1c") {
            selectedKey = "1c"
        } else if strings.Contains(q, "telegram") || strings.Contains(q, "телеграм") {
            selectedKey = "telegram"
        } else if strings.Contains(q, "whatsapp") {
            selectedKey = "whatsapp"
        } else if strings.Contains(q, "bitrix") {
            selectedKey = "bitrix"
        } else if strings.Contains(q, "магазин") || strings.Contains(q, "интернет") {
            selectedKey = "shop"
        } else if strings.Contains(q, "seo") {
            selectedKey = "seo"
        } else if strings.Contains(q, "hr") {
            selectedKey = "hr"
        } else if strings.Contains(q, "финанс") {
            selectedKey = "finance"
        } else if strings.Contains(q, "логистик") {
            selectedKey = "logistics"
        }
        
        if selectedKey != "" {
            s := servicesList[selectedKey]
            state.TempData["service"] = selectedKey
            state.TempData["service_name"] = s["name"].(string)
            state.TempData["service_price"] = fmt.Sprintf("%d", s["price"])
            state.AwaitingPhone = true
            return fmt.Sprintf("✅ **Оформление заказа: %s**\n\n💰 Стоимость: %d ₽/%s\n%s\n\n📞 Для оформления оставьте ваш номер телефона:", 
                s["name"], s["price"], s["period"], s["description"])
        }
        
        var list strings.Builder
        list.WriteString("Что именно хотите заказать?\n\n")
        for _, s := range servicesList {
            list.WriteString(fmt.Sprintf("• %s - %d ₽/%s\n", s["name"], s["price"], s["period"]))
        }
        list.WriteString("\nНапишите: **'хочу CRM'** или **'хочу VPN'**")
        return list.String()
    }

    // ДОБАВЛЕНИЕ КЛИЕНТА
    if strings.Contains(q, "добавь клиент") || strings.Contains(q, "создай клиент") {
        re := regexp.MustCompile(`(?:клиент[а]?\s+)([А-Яа-яA-Za-z0-9\s]+)`)
        matches := re.FindStringSubmatch(q)
        if len(matches) > 1 {
            name := strings.TrimSpace(matches[1])
            _, err := database.Pool.Exec(ctx, "INSERT INTO customers (name, created_by, created_at) VALUES ($1, $2, NOW())", name, userName)
            if err != nil {
                return fmt.Sprintf("❌ Ошибка: %v", err)
            }
            return fmt.Sprintf("✅ Клиент **%s** успешно добавлен в CRM!\n\n📍 Посмотреть: /crm", name)
        }
        return fmt.Sprintf("%s, напишите: **'добавь клиента ООО Ромашка'**", userName)
    }

    // ПОКАЗАТЬ КЛИЕНТОВ
    if strings.Contains(q, "покажи клиент") || strings.Contains(q, "список клиент") {
        rows, err := database.Pool.Query(ctx, "SELECT id, name, created_at FROM customers ORDER BY created_at DESC LIMIT 10")
        if err != nil {
            return "❌ Ошибка загрузки клиентов"
        }
        defer rows.Close()
        
        var result strings.Builder
        result.WriteString("📊 **Список клиентов в CRM:**\n\n")
        count := 0
        for rows.Next() {
            var id int
            var name string
            var createdAt time.Time
            rows.Scan(&id, &name, &createdAt)
            result.WriteString(fmt.Sprintf("• %s (ID: %d, создан: %s)\n", name, id, createdAt.Format("02.01.2006")))
            count++
        }
        if count == 0 {
            return "📊 В CRM пока нет клиентов.\n\nДобавьте: **'добавь клиента ООО Ромашка'**"
        }
        return result.String()
    }

    // ПОКАЗАТЬ СДЕЛКИ
    if strings.Contains(q, "покажи сделк") || strings.Contains(q, "список сделк") {
        rows, err := database.Pool.Query(ctx, "SELECT id, name, status, created_at FROM deals ORDER BY created_at DESC LIMIT 10")
        if err != nil {
            return "❌ Ошибка загрузки сделок"
        }
        defer rows.Close()
        
        var result strings.Builder
        result.WriteString("📊 **Список сделок:**\n\n")
        for rows.Next() {
            var id int
            var name, status string
            var createdAt time.Time
            rows.Scan(&id, &name, &status, &createdAt)
            result.WriteString(fmt.Sprintf("• %s (статус: %s, создана: %s)\n", name, status, createdAt.Format("02.01.2006")))
        }
        return result.String()
    }

    // СОЗДАНИЕ КОМАНДЫ
    if strings.Contains(q, "создай команд") || strings.Contains(q, "новая команд") {
        re := regexp.MustCompile(`(?:команду?\s+)([А-Яа-яA-Za-z0-9\s]+)`)
        matches := re.FindStringSubmatch(q)
        if len(matches) > 1 {
            teamName := strings.TrimSpace(matches[1])
            _, err := database.Pool.Exec(ctx, "INSERT INTO teams (name, created_by, created_at) VALUES ($1, $2, NOW())", teamName, userName)
            if err != nil {
                return fmt.Sprintf("❌ Ошибка: %v", err)
            }
            return fmt.Sprintf("✅ Команда **%s** создана!\n\n👥 Добавить участников: /team/team\n📋 Задачи: /tasks\n💬 Чат: /chat", teamName)
        }
        return fmt.Sprintf("%s, напишите: **'создай команду Маркетинг'**", userName)
    }

    // ПОИСК В ИНТЕРНЕТЕ
    if strings.Contains(q, "найди") || strings.Contains(q, "поищи") {
        searchQuery := strings.TrimPrefix(q, "найди")
        searchQuery = strings.TrimPrefix(searchQuery, "поищи")
        searchQuery = strings.TrimSpace(searchQuery)
        
        if len(searchQuery) > 5 {
            results, err := searchInternet(searchQuery)
            if err != nil {
                return fmt.Sprintf("❌ Ошибка поиска: %v", err)
            }
            if len(results) > 0 {
                var res strings.Builder
                res.WriteString(fmt.Sprintf("🔍 **Результаты поиска по запросу '%s':**\n\n", searchQuery))
                for i, r := range results {
                    if i >= 3 {
                        break
                    }
                    if r.Title != "" {
                        res.WriteString(fmt.Sprintf("• %s\n", r.Title))
                    }
                    if r.Description != "" {
                        res.WriteString(fmt.Sprintf("  %s\n\n", truncateText(r.Description, 150)))
                    }
                }
                return res.String()
            }
            return fmt.Sprintf("🔍 По запросу '%s' ничего не найдено.", searchQuery)
        }
        return "🔍 Что именно найти? Например: **'найди информацию о разработке сайтов'**"
    }

    // ПОМОЩЬ
    if strings.Contains(q, "помощь") || strings.Contains(q, "что умее") || strings.Contains(q, "help") {
        return fmt.Sprintf(`🤖 **Что я умею, %s:**

💰 **Узнать цены:**
• "сколько стоит CRM"
• "цена VPN"

🛒 **Купить услугу:**
• "хочу купить CRM"
• "заказать VPN"

📊 **Работа с CRM:**
• "добавь клиента ООО Ромашка"
• "покажи клиентов"
• "покажи сделки"

👥 **TeamSphere:**
• "создай команду Маркетинг"

🔍 **Поиск в интернете:**
• "найди информацию о разработке сайтов"

❓ **Другое:**
• "как заполнить CRM"
• "что умеет система"

Просто напишите, что хотите сделать!`, userName)
    }

    // КАК ЗАПОЛНИТЬ CRM
    if strings.Contains(q, "как заполнить crm") {
        return `📝 **Как заполнить CRM:**

1️⃣ **Через AI (быстро):**
   • "добавь клиента ООО Ромашка"
   • "покажи клиентов"

2️⃣ **Вручную:**
   • Перейдите в раздел CRM: /crm
   • Нажмите "Добавить клиента"
   • Заполните поля

3️⃣ **Импорт из Excel:**
   • CRM → Импорт → Загрузите файл

Что хотите сделать?`
    }

    // О ПРОЕКТЕ
    if strings.Contains(q, "что умеет система") || strings.Contains(q, "о проекте") || strings.Contains(q, "возможности") {
        return `🚀 **SaaSPro - возможности платформы:**

📊 **CRM** - управление клиентами, сделки, воронка
🔒 **VPN** - безопасный доступ (Stealth режим)
🧠 **AI ассистент** - умный помощник с YandexGPT
📦 **Маркетплейс** - приложения и интеграции
🔌 **Интеграции** - 1С, Bitrix24, Telegram, WhatsApp
💰 **Финансы** - учет, платежи, отчетность
🚚 **Логистика** - управление доставкой
👥 **HR** - управление персоналом
📈 **Аналитика** - дашборды и отчеты
☁️ **Облако** - файловое хранилище

📍 Все доступно в веб-интерфейсе!

Что вас интересует?`
    }

    // ПО УМОЛЧАНИЮ
    return fmt.Sprintf(`%s, я AI-ассистент SaaSPro.

Вот что могу:

💰 **Цены** - "сколько стоит CRM"
🛒 **Купить** - "хочу купить VPN"
📊 **CRM** - "добавь клиента"
👥 **Team** - "создай команду"
🔍 **Поиск** - "найди информацию о..."
❓ **Помощь** - "что умеешь"

Что хотите сделать?`, userName)
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
