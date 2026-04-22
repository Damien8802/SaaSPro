package services

import (
    "regexp"
    "strings"
)

type IntentExtended struct {
    Type            string
    Action          string
    Module          string
    RequiresPayment bool
    Entities        map[string]string
    Confidence      float64
}

func AnalyzeIntentExtended(message string) (IntentExtended, map[string]string) {
    message = strings.ToLower(message)
    entities := make(map[string]string)

    // Создание клиента
    if match := regexp.MustCompile(`(создай|добавь|новый|нового)\s+клиент[а]?\s+([А-Яа-я\s]+)`).FindStringSubmatch(message); len(match) > 2 {
        entities["name"] = strings.TrimSpace(match[2])
        
        phoneRe := regexp.MustCompile(`(\+?7[\d\s()-]{10,})`)
        if phoneMatch := phoneRe.FindStringSubmatch(message); len(phoneMatch) > 0 {
            entities["phone"] = phoneMatch[0]
        }
        
        return IntentExtended{
            Type:       "crm",
            Action:     "create_customer",
            Module:     "crm",
            Confidence: 0.95,
        }, entities
    }

    // Создание сделки
    if match := regexp.MustCompile(`(создай|добавь)\s+сделк[уа]?\s+([А-Яа-я\s]+)`).FindStringSubmatch(message); len(match) > 2 {
        entities["deal_name"] = strings.TrimSpace(match[2])
        
        amountRe := regexp.MustCompile(`(\d+[\s]?\d*)[\s]?(тыс|т|милл|млн|₽|руб)?`)
        if amountMatch := amountRe.FindStringSubmatch(message); len(amountMatch) > 1 {
            amount := strings.ReplaceAll(amountMatch[1], " ", "")
            entities["amount"] = amount
        }
        
        return IntentExtended{
            Type:       "crm",
            Action:     "create_deal",
            Module:     "crm",
            Confidence: 0.9,
        }, entities
    }

    // Выставление счёта
    if match := regexp.MustCompile(`(выстав[ьи]|создай)\s+счёт\s+([А-Яа-я\s]+)\s+на\s+(\d+)`).FindStringSubmatch(message); len(match) > 3 {
        entities["customer_name"] = strings.TrimSpace(match[2])
        entities["amount"] = match[3]
        
        return IntentExtended{
            Type:       "fincore",
            Action:     "create_invoice",
            Module:     "fincore",
            Confidence: 0.95,
        }, entities
    }

    // Создание задачи
    if match := regexp.MustCompile(`(создай|добавь)\s+задач[уа]?\s+([А-Яа-я\s]+)\s+(для|исполнителю)\s+([А-Яа-я\s]+)`).FindStringSubmatch(message); len(match) > 4 {
        entities["title"] = strings.TrimSpace(match[2])
        entities["assignee"] = strings.TrimSpace(match[4])
        
        return IntentExtended{
            Type:       "teamsphere",
            Action:     "create_task",
            Module:     "teamsphere",
            Confidence: 0.95,
        }, entities
    }

    // Помощь
    if match := regexp.MustCompile(`(помощь|что умеешь|как использовать|help)`).FindString(message); match != "" {
        return IntentExtended{
            Type:       "general",
            Action:     "help",
            Module:     "general",
            Confidence: 1.0,
        }, entities
    }

    return IntentExtended{
        Type:       "general",
        Action:     "chat",
        Module:     "general",
        Confidence: 0.5,
    }, entities
}

func GetHelpMessage() string {
    return `🤖 **Я - автономный AI-ассистент**

📋 **CRM:**
• "Создай клиента ООО Ромашка +7 999 123-45-67"
• "Создай сделку для Ромашки на 500 000"

💰 **Финансы:**
• "Выставь счёт Ромашке на 500 000"

✅ **Задачи:**
• "Создай задачу Позвонить клиенту для Иванова"

Что нужно сделать?`
}