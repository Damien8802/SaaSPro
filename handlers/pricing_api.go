package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type Tariff struct {
    Name        string   `json:"name"`
    Price       int      `json:"price"`
    YearlyPrice int      `json:"yearly_price"`
    Users       string   `json:"users"`
    Features    []string `json:"features"`
    Popular     bool     `json:"popular"`
    Family      bool     `json:"family"`
}

// GetTariffs возвращает список тарифов
func GetTariffs(c *gin.Context) {
    tariffs := []Tariff{
        {
            Name:        "Базовый",
            Price:       990,
            YearlyPrice: 9900,
            Users:       "1 пользователь",
            Features:    []string{"Базовая аналитика", "Email поддержка", "10 AI-запросов/день", "Базовая модель AI"},
            Popular:     false,
            Family:      false,
        },
        {
            Name:        "Семейный",
            Price:       2490,
            YearlyPrice: 24900,
            Users:       "До 5 пользователей",
            Features:    []string{"Продвинутая аналитика", "Приоритетная поддержка", "50 AI-запросов/день", "Продвинутая модель AI", "Детский режим", "Общая библиотека"},
            Popular:     false,
            Family:      true,
        },
        {
            Name:        "Профессиональный",
            Price:       2990,
            YearlyPrice: 29900,
            Users:       "До 1000 пользователей",
            Features:    []string{"Продвинутая аналитика", "Приоритетная поддержка", "CRM", "100 AI-запросов/день", "Продвинутая модель AI", "Загрузка файлов"},
            Popular:     true,
            Family:      false,
        },
        {
            Name:        "Корпоративный",
            Price:       9990,
            YearlyPrice: 99900,
            Users:       "Неограниченно",
            Features:    []string{"Полная аналитика", "24/7 поддержка", "Все интеграции", "1000 AI-запросов/день", "Экспертная модель AI", "Загрузка файлов", "Голосовой ввод"},
            Popular:     false,
            Family:      false,
        },
    }
    c.JSON(http.StatusOK, tariffs)
}