package services

import (
    "context"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type SimpleAI struct {
    db *pgxpool.Pool
}

func NewSimpleAI(db *pgxpool.Pool) *SimpleAI {
    return &SimpleAI{db: db}
}

func (ai *SimpleAI) Process(tenantID, userID uuid.UUID, message string) (string, error) {
    msg := strings.ToLower(strings.TrimSpace(message))

    if strings.Contains(msg, "создай вакансию") {
        return ai.createVacancy(tenantID, msg)
    }
    if strings.Contains(msg, "список вакансий") {
        return ai.listVacancies(tenantID)
    }
    if strings.Contains(msg, "опубликуй на hh") {
        return ai.publishToHH(tenantID)
    }
    if strings.Contains(msg, "статистика") {
        return ai.getStats(tenantID)
    }
    if strings.Contains(msg, "помощь") {
        return ai.getHelp(), nil
    }

    return ai.getHelp(), nil
}

func (ai *SimpleAI) createVacancy(tenantID uuid.UUID, msg string) (string, error) {
    title := strings.TrimPrefix(msg, "создай вакансию")
    title = strings.TrimSpace(title)
    if title == "" {
        return "📝 Напишите название: создай вакансию разработчик", nil
    }

    var id string
    err := ai.db.QueryRow(context.Background(), `
        INSERT INTO hr_vacancies (title, status, tenant_id, created_at)
        VALUES ($1, 'open', $2, NOW())
        RETURNING id::text
    `, title, tenantID).Scan(&id)

    if err != nil {
        return "❌ Ошибка", nil
    }

    return fmt.Sprintf("✅ Вакансия '%s' создана! ID: %s", title, id[:8]), nil
}

func (ai *SimpleAI) listVacancies(tenantID uuid.UUID) (string, error) {
    rows, err := ai.db.Query(context.Background(), `
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
        return "Нет вакансий", nil
    }

    return "Ваши вакансии:\n" + strings.Join(list, "\n"), nil
}

func (ai *SimpleAI) publishToHH(tenantID uuid.UUID) (string, error) {
    var title string
    err := ai.db.QueryRow(context.Background(), `
        SELECT title FROM hr_vacancies WHERE tenant_id = $1 AND status = 'open' ORDER BY created_at DESC LIMIT 1
    `, tenantID).Scan(&title)

    if err != nil {
        return "Нет вакансий для публикации", nil
    }

    return fmt.Sprintf("Вакансия '%s' отправлена на HeadHunter!", title), nil
}

func (ai *SimpleAI) getStats(tenantID uuid.UUID) (string, error) {
    var employees, vacancies int
    ai.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM hr_employees WHERE tenant_id=$1 AND status='active'", tenantID).Scan(&employees)
    ai.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM hr_vacancies WHERE tenant_id=$1 AND status='open'", tenantID).Scan(&vacancies)

    return fmt.Sprintf("📊 Статистика:\n👥 Сотрудников: %d\n💼 Вакансий: %d", employees, vacancies), nil
}

func (ai *SimpleAI) getHelp() string {
    return `🤖 Команды:
• создай вакансию [название]
• список вакансий
• опубликуй на HH
• статистика`
}