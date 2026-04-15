package services

import (
    "fmt"
    "strings"
)

type PriceSearchService struct {
    yandexAPIKey string
}

func NewPriceSearchService(yandexAPIKey, yandexFolderID string) *PriceSearchService {
    return &PriceSearchService{
        yandexAPIKey: yandexAPIKey,
    }
}

type PriceResult struct {
    Query        string  `json:"query"`
    AvgPrice     float64 `json:"avg_price"`
    MinPrice     float64 `json:"min_price"`
    MaxPrice     float64 `json:"max_price"`
    SourcesCount int     `json:"sources_count"`
    Cached       bool    `json:"cached"`
    Message      string  `json:"message"`
}

func (s *PriceSearchService) SearchPrice(serviceType string) (*PriceResult, error) {
    basePrice := getBasePrice(serviceType)
    
    return &PriceResult{
        Query:        serviceType,
        AvgPrice:     basePrice,
        MinPrice:     basePrice * 0.7,
        MaxPrice:     basePrice * 1.5,
        SourcesCount: 1,
        Cached:       false,
        Message:      fmt.Sprintf("📊 Средняя цена разработки \"%s\": %.0f ₽", serviceType, basePrice),
    }, nil
}

func getBasePrice(serviceType string) float64 {
    basePrices := map[string]float64{
        "телеграм бот":     50000,
        "чат бот":          45000,
        "интернет-магазин": 150000,
        "crm":              200000,
        "лендинг":          30000,
        "сайт":             50000,
        "приложение":       300000,
    }
    
    for key, price := range basePrices {
        if strings.Contains(strings.ToLower(serviceType), key) {
            return price
        }
    }
    return 50000
}

func FormatPriceMessage(serviceType string, price float64) string {
    return fmt.Sprintf(`🔍 **Поиск цен на "%s"**

📊 **Ориентировочная стоимость:** %.0f ₽
📉 Минимальная: %.0f ₽
📈 Максимальная: %.0f ₽

💡 Хотите заказать? Напишите "Хочу заказать" и я помогу оформить заявку!`, serviceType, price, price*0.7, price*1.5)
}
