package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	fmt.Println("🔑 Пробуем без токена...")

	requestBody := map[string]interface{}{
		"model": "gpt-4o-mini", // или "DeepSeek-R1"
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Привет! Скажи 'GitHub Models работает' на русском",
			},
		},
		"max_tokens": 100,
	}

	jsonBody, _ := json.Marshal(requestBody)

	// Пробуем без токена
	req, _ := http.NewRequest("POST", "https://models.inference.ai.azure.com/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// НЕ добавляем Authorization

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Ошибка: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Статус: %d\nОтвет: %s\n", resp.StatusCode, string(body))
}