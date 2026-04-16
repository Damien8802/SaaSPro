package services

import (
    "log"
)

// SearchResult - результат поиска
type SearchResult struct {
    Title   string  `json:"title"`
    URL     string  `json:"url"`
    Price   float64 `json:"price"`
    Description string `json:"description"`
}

// SearchWeb - поиск в интернете
func SearchWeb(query string) ([]SearchResult, error) {
    log.Printf("🔍 Поисковый запрос: %s", query)
    return []SearchResult{}, nil
}
