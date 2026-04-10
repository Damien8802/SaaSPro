
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/joho/godotenv"
    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    godotenv.Load()
    
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        dbURL = "postgres://postgres:6213110@localhost:5432/GO?sslmode=disable"
    }

    pool, err := pgxpool.New(context.Background(), dbURL)
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Добавляем тестового пользователя
    _, err = pool.Exec(context.Background(), `
        INSERT INTO users (id, email, name, role, created_at)
        VALUES (
            'aa5f14e6-30e1-476c-ac42-8c11ced838a4',
            'dev@saaspro.local',
            'Development User',
            'admin',
            NOW()
        ) ON CONFLICT (id) DO NOTHING;
    `)

    if err != nil {
        fmt.Println("Ошибка:", err)
    } else {
        fmt.Println("✅ Пользователь добавлен/уже существует")
    }

    // Проверим
    var id, email, name string
    pool.QueryRow(context.Background(), 
        "SELECT id, email, name FROM users WHERE id = $1", 
        "aa5f14e6-30e1-476c-ac42-8c11ced838a4").Scan(&id, &email, &name)
    
    if id != "" {
        fmt.Printf("Пользователь найден: %s (%s) - %s\n", name, email, id)
    }
}
'@ | Out-File -FilePath "add_dev_user.go" -Encoding utf8