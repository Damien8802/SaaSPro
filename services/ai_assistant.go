package services

import (
    "fmt"
    "regexp"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type AIAssistant struct {
    db *pgxpool.Pool
}

func NewAIAssistant(db *pgxpool.Pool) *AIAssistant {
    return &AIAssistant{
        db: db,
    }
}

// ProcessRequest - основной обработчик
func (a *AIAssistant) ProcessRequest(tenantID, userID uuid.UUID, message string) (string, error) {
    message = strings.ToLower(message)

    // Создание сделки
    if strings.Contains(message, "создай сделку") || strings.Contains(message, "новая сделк") {
        params := a.extractDealData(message)
        return a.createDeal(tenantID, userID, params)
    }

    // Поиск клиентов
    if strings.Contains(message, "найди клиента") || strings.Contains(message, "покажи клиентов") {
        searchTerm := a.extractSearchTerm(message)
        return a.findCustomers(tenantID, searchTerm)
    }

    // Список сделок
    if strings.Contains(message, "покажи сделки") || strings.Contains(message, "список сделок") {
        return a.getDeals(tenantID)
    }

    // Создание задачи
    if strings.Contains(message, "создай задач") || strings.Contains(message, "новая задача") {
        return a.createTask(tenantID, userID, message)
    }

    // Покажи задачи
    if strings.Contains(message, "покажи задачи") || strings.Contains(message, "список задач") {
        return a.getTasks(tenantID, userID)
    }

    // Бухгалтерия - проводки
    if strings.Contains(message, "проводк") || strings.Contains(message, "бухгалтери") {
        return "📝 Для создания проводок перейдите в раздел Финансы → Журнал проводок", nil
    }

    // ОСВ
    if strings.Contains(message, "осв") || strings.Contains(message, "оборотно-сальдовая") {
        return "📊 Отчёт ОСВ формируется в разделе Финансы → Оборотно-сальдовая ведомость", nil
    }

    // Помощь
    if strings.Contains(message, "помощь") || strings.Contains(message, "что ты умеешь") || strings.Contains(message, "команды") {
        return a.getHelp(), nil
    }

    return "👋 Я AI-ассистент SaaSPro. Напишите **\"помощь\"** для списка команд.", nil
}

// createDeal - создание сделки
func (a *AIAssistant) createDeal(tenantID, userID uuid.UUID, params map[string]interface{}) (string, error) {
    name, _ := params["name"].(string)
    if name == "" {
        return "❌ Укажите название компании. Например: «создай сделку для ООО Ромашка»", nil
    }

    amount := 0.0
    if val, ok := params["amount"].(float64); ok {
        amount = val
    }

    dealID := uuid.New()
    _, err := a.db.Exec(nil, `
        INSERT INTO deals (id, tenant_id, title, value, stage, created_by, created_at, updated_at)
        VALUES ($1, $2, $3, $4, 'lead', $5, NOW(), NOW())
    `, dealID, tenantID, name, amount, userID)

    if err != nil {
        return fmt.Sprintf("❌ Не удалось создать сделку: %v", err), nil
    }

    amountStr := ""
    if amount > 0 {
        amountStr = fmt.Sprintf(" на сумму %.2f ₽", amount)
    }

    return fmt.Sprintf("✅ **Сделка создана!**\n\n📝 Название: %s%s\n\n💡 Чтобы посмотреть сделку, перейдите в раздел CRM.", name, amountStr), nil
}

// findCustomers - поиск клиентов
func (a *AIAssistant) findCustomers(tenantID uuid.UUID, searchTerm string) (string, error) {
    if searchTerm == "" {
        return "🔍 Введите имя или email для поиска. Например: «найди клиента Иванов»", nil
    }

    rows, err := a.db.Query(nil, `
        SELECT name, email, phone FROM customers
        WHERE tenant_id = $1 AND (name ILIKE $2 OR email ILIKE $2)
        LIMIT 5
    `, tenantID, "%"+searchTerm+"%")
    if err != nil {
        return "❌ Ошибка поиска клиентов", nil
    }
    defer rows.Close()

    var customers []string
    for rows.Next() {
        var name, email, phone string
        rows.Scan(&name, &email, &phone)
        customers = append(customers, fmt.Sprintf("• %s (%s, %s)", name, email, phone))
    }

    if len(customers) == 0 {
        return fmt.Sprintf("🔍 Клиенты по запросу «%s» не найдены.", searchTerm), nil
    }

    return fmt.Sprintf("🔍 **Найдено клиентов: %d**\n\n%s", len(customers), strings.Join(customers, "\n")), nil
}

// getDeals - получение сделок
func (a *AIAssistant) getDeals(tenantID uuid.UUID) (string, error) {
    rows, err := a.db.Query(nil, `
        SELECT title, value, stage FROM deals
        WHERE tenant_id = $1
        ORDER BY created_at DESC
        LIMIT 10
    `, tenantID)
    if err != nil {
        return "❌ Ошибка получения сделок", nil
    }
    defer rows.Close()

    var deals []string
    for rows.Next() {
        var title string
        var value float64
        var stage string
        rows.Scan(&title, &value, &stage)
        deals = append(deals, fmt.Sprintf("• %s - %.2f ₽ (%s)", title, value, stage))
    }

    if len(deals) == 0 {
        return "📊 У вас пока нет сделок.", nil
    }

    return fmt.Sprintf("📊 **Ваши сделки:**\n\n%s", strings.Join(deals, "\n")), nil
}

// createTask - создание задачи
func (a *AIAssistant) createTask(tenantID, userID uuid.UUID, message string) (string, error) {
    return "✅ Создание задач доступно в разделе TeamSphere → Задачи\n\n💡 Чтобы создать задачу, перейдите в соответствующий раздел.", nil
}

// getTasks - получение задач
func (a *AIAssistant) getTasks(tenantID, userID uuid.UUID) (string, error) {
    return "📋 Список задач доступен в разделе TeamSphere → Мои задачи\n\n💡 Для просмотра всех задач перейдите в раздел TeamSphere.", nil
}

// extractDealData - извлечение данных из текста
func (a *AIAssistant) extractDealData(message string) map[string]interface{} {
    params := make(map[string]interface{})
    message = strings.ToLower(message)

    // Извлечение названия компании
    companyRe := regexp.MustCompile(`(?:для|клиента)\s+([А-Яа-яA-Za-z0-9\s]+?)(?:\s+на\s+|\s*$)`)
    if match := companyRe.FindStringSubmatch(message); len(match) > 1 {
        params["name"] = strings.TrimSpace(match[1])
    }

    // Извлечение суммы
    amountRe := regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(?:руб|р)`)
    if match := amountRe.FindStringSubmatch(message); len(match) > 1 {
        amountStr := strings.Replace(match[1], ",", ".", -1)
        var amount float64
        fmt.Sscanf(amountStr, "%f", &amount)
        params["amount"] = amount
    }

    return params
}

// extractSearchTerm - извлечение поискового запроса
func (a *AIAssistant) extractSearchTerm(message string) string {
    re := regexp.MustCompile(`клиента\s+([А-Яа-яA-Za-z]+)`)
    match := re.FindStringSubmatch(message)
    if len(match) > 1 {
        return match[1]
    }
    return ""
}

// getHelp - справка
func (a *AIAssistant) getHelp() string {
    return `🤖 **AI Assistant SaaSPro**

**Что я умею:**

📊 **CRM:**
• "создай сделку для ООО Ромашка на 1.5 млн"
• "найди клиента Иванов"
• "покажи мои сделки"

💰 **FinCore (Бухгалтерия):**
• "покажи ОСВ"
• "создай проводку"

✅ **TeamSphere (Задачи):**
• "создай задачу"
• "покажи мои задачи"

👥 **HR:**
• "оформи отпуск"
• "список сотрудников"

☁️ **Nebula Cloud:**
• "загрузи файл"
• "мои файлы"

📈 **Аналитика:**
• "покажи отчёты"

Чем могу помочь?`
}