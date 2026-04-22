package handlers

import (
    "log"
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
    "subscription-system/services"
)

func YandexHRHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    userID := c.GetString("user_id")
    if userID == "" {
        userID = "11111111-1111-1111-1111-111111111111"
    }

    var req struct {
        Message string `json:"message"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"reply": "Неверный формат"})
        return
    }

    log.Printf("🤖 YandexHR запрос: %s", req.Message)

    apiKey := os.Getenv("YANDEX_API_KEY")
    folderID := os.Getenv("YANDEX_FOLDER_ID")

    if apiKey == "" || folderID == "" {
        c.JSON(http.StatusOK, gin.H{"reply": "❌ YandexGPT не настроен"})
        return
    }

    ai := services.NewYandexHRAssistant(database.Pool, apiKey, folderID)

    tenantUUID, _ := uuid.Parse(tenantID)
    userUUID, _ := uuid.Parse(userID)

    reply, err := ai.Process(tenantUUID, userUUID, req.Message)
    if err != nil {
        log.Printf("❌ Ошибка: %v", err)
        c.JSON(http.StatusOK, gin.H{"reply": "❌ Ошибка обработки"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"reply": reply})
}