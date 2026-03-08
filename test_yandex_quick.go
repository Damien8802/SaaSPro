package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	folderID := os.Getenv("YANDEX_FOLDER_ID")
	apiKey := os.Getenv("YANDEX_API_KEY")

	if folderID == "" || apiKey == "" {
		fmt.Println("❌ YANDEX_FOLDER_ID или YANDEX_API_KEY не найдены в .env")
		return
	}

	fmt.Println("🔑 Тестируем YandexGPT...")
	fmt.Printf("Folder ID: %s\n", folderID)
	fmt.Printf("API Key: %s...\n", apiKey[:10])

	// YandexGPT API endpoint
	url := fmt.Sprintf("https://llm.api.cloud.yandex.net/foundationModels/v1/completion")

	requestBody := map[string]interface{}{
		"modelUri": fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
		"completionOptions": map[string]interface{}{
			"stream":      false,
			"temperature": 0.7,
			"maxTokens":   100,
		},
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Привет! Скажи 'YandexGPT работает' на русском. Ответь одним предложением.",
			},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Ошибка подключения: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		fmt.Println("✅ YandexGPT работает!")
		
		var response struct {
			Result struct {
				Alternatives []struct {
					Message struct {
						Text string `json:"text"`
					} `json:"message"`
				} `json:"alternatives"`
			} `json:"result"`
		}
		json.Unmarshal(body, &response)
		
		if len(response.Result.Alternatives) > 0 {
			fmt.Printf("🤖 Ответ: %s\n", response.Result.Alternatives[0].Message.Text)
		}
	} else {
		fmt.Printf("❌ Ошибка %d: %s\n", resp.StatusCode, string(body))
	}
}