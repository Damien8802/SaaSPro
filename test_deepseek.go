package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	apiKey := "sk-095c078712be40f397d8373645b0e3d8"
	
	fmt.Println("🔑 Тестируем DeepSeek API...")

	requestBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Привет! Скажи 'DeepSeek работает' на русском",
			},
		},
		"temperature": 0.7,
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Ошибка подключения: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		fmt.Println("✅ DeepSeek API работает!")
		
		var response struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		json.Unmarshal(body, &response)
		
		if len(response.Choices) > 0 {
			fmt.Printf("🤖 Ответ: %s\n", response.Choices[0].Message.Content)
		}
	} else {
		fmt.Printf("❌ Ошибка %d: %s\n", resp.StatusCode, string(body))
	}
}