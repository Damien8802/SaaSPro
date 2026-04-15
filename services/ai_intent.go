package services

import (
    "context"
    "fmt"
    "os"
    "strings"
    "time"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func InitDB(pool *pgxpool.Pool) {
    dbPool = pool
    fmt.Println("✅ AI Intent DB инициализирован")
}

func ProcessMessage(sessionID, message string) (string, bool) {
    manager := GetConversationManager()
    state := manager.GetState(sessionID)
    message = strings.TrimSpace(message)
    
    if state == nil {
        state = &ConversationState{
            Step:       "greeting",
            LastUpdate: time.Now(),
        }
        manager.SetState(sessionID, state)
        return GetTimeGreeting() + "\n\nКак к вам обращаться?", false
    }
    
    switch state.Step {
    case "greeting":
        if len(message) > 2 && !strings.ContainsAny(message, "0123456789") {
            state.UserName = message
            state.Step = "ask_service"
            manager.SetState(sessionID, state)
            return fmt.Sprintf("Приятно познакомиться, %s! 👋\n\nЧем я могу вам помочь?\n\n🔹 Узнать цену разработки\n🔹 Связь с разработчиком\n\nНапишите, что вас интересует:", state.UserName), false
        }
        return "Как к вам обращаться? Напишите ваше имя.", false
        
    case "ask_service":
        lowerMsg := strings.ToLower(message)
        if strings.Contains(lowerMsg, "цен") || strings.Contains(lowerMsg, "стоимость") {
            state.Step = "ask_service_type"
            manager.SetState(sessionID, state)
            return "Какой продукт вас интересует?\n\n• Телеграм бот\n• Интернет-магазин\n• CRM система\n• Сайт/лендинг\n\nНапишите:", false
        } else if strings.Contains(lowerMsg, "разработчик") {
            state.Step = "ask_contact"
            manager.SetState(sessionID, state)
            return "Хорошо! Оставьте ваш контакт (телефон или Telegram):", false
        } else {
            return "Я могу помочь:\n• Рассчитать стоимость разработки\n• Связать с разработчиком\n\nЧто вас интересует?", false
        }
        
    case "ask_service_type":
        state.Service = message
        state.Step = "ask_deadline"
        manager.SetState(sessionID, state)
        return fmt.Sprintf("Отлично! 📅 Какой срок разработки %s вас интересует?\n\n• Как можно быстрее\n• 1-2 недели\n• 3-4 недели\n• 1-2 месяца\n\nНапишите:", state.Service), false
        
    case "ask_deadline":
        state.Deadline = message
        state.Step = "ask_contact"
        manager.SetState(sessionID, state)
        return "Понял! ✅\n\n📝 Оставьте ваш контактный телефон или Telegram:", false
        
    case "ask_contact":
        state.UserContact = message
        state.Step = "done"
        manager.SetState(sessionID, state)
        
        // Сохраняем в БД
        go saveOrderToDB(state)
        
        manager.ClearState(sessionID)
        return "✅ **Заявка отправлена!**\n\nС вами свяжутся в течение 15 минут.\n\nВсего наилучшего! 🚀", true
    }
    
    return "Извините, я не понял. Напишите 'привет' чтобы начать заново!", false
}

func saveOrderToDB(state *ConversationState) {
    if dbPool == nil {
        fmt.Println("❌ База данных не инициализирована, сохраняю в файл")
        saveOrderToFile(state)
        return
    }
    
    ctx := context.Background()
    query := `INSERT INTO service_orders (client_name, client_contact, service_type, deadline, status) 
              VALUES ($1, $2, $3, $4, 'new')`
    
    _, err := dbPool.Exec(ctx, query, state.UserName, state.UserContact, state.Service, state.Deadline)
    if err != nil {
        fmt.Printf("❌ Ошибка сохранения в БД: %v\n", err)
        saveOrderToFile(state)
    } else {
        fmt.Printf("✅ Заявка от %s сохранена в БД!\n", state.UserName)
    }
}

func saveOrderToFile(state *ConversationState) {
    os.MkdirAll("orders", 0755)
    
    filename := fmt.Sprintf("orders/заявка_%s_%s.txt", 
        time.Now().Format("20060102_150405"),
        strings.ReplaceAll(state.UserName, " ", "_"))
    
    content := fmt.Sprintf(`НОВАЯ ЗАЯВКА
================
Клиент: %s
Контакт: %s
Услуга: %s
Срок: %s
Дата: %s
================`,
        state.UserName,
        state.UserContact,
        state.Service,
        state.Deadline,
        time.Now().Format("02.01.2006 15:04:05"))
    
    os.WriteFile(filename, []byte(content), 0644)
    fmt.Printf("📁 Заявка сохранена в файл: %s\n", filename)
}

func GetTimeGreeting() string {
    hour := time.Now().Hour()
    switch {
    case hour < 12:
        return "🌅 Доброе утро!"
    case hour < 18:
        return "☀️ Добрый день!"
    default:
        return "🌙 Добрый вечер!"
    }
}
