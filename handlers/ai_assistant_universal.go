package handlers

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
    
    "subscription-system/services"
)

type UniversalAIAssistant struct{}

func NewUniversalAIAssistant(yandexAPIKey, yandexFolderID, telegramBotToken, telegramChatID, adminChatID string, pool *pgxpool.Pool) *UniversalAIAssistant {
    services.InitDB(pool)
    return &UniversalAIAssistant{}
}

func (ai *UniversalAIAssistant) ChatHandler(c *gin.Context) {
    var req struct {
        Message   string `json:"message"`
        SessionID string `json:"session_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.SessionID == "" {
        req.SessionID = c.ClientIP()
    }
    
    response, isDone := services.ProcessMessage(req.SessionID, req.Message)
    
    c.JSON(http.StatusOK, gin.H{
        "response":   response,
        "is_done":    isDone,
        "session_id": req.SessionID,
    })
}

func (ai *UniversalAIAssistant) GetHistory(c *gin.Context) {
    c.JSON(http.StatusOK, []gin.H{})
}

func (ai *UniversalAIAssistant) GetActions(c *gin.Context) {
    actions := []gin.H{
        {"action": "start", "name": "Начать диалог", "example": "Привет"},
        {"action": "price", "name": "Узнать цену", "example": "Узнать цену разработки"},
    }
    c.JSON(http.StatusOK, actions)
}

func (ai *UniversalAIAssistant) GetSettings(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "search_enabled": true,
        "temperature":    0.7,
    })
}
