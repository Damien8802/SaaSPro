package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
    "subscription-system/services"
)

func AIAssistantHandler(c *gin.Context) {
    tenantIDRaw, exists := c.Get("tenant_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    userIDRaw, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    tenantID, _ := uuid.Parse(tenantIDRaw.(string))
    userID, _ := uuid.Parse(userIDRaw.(string))

    var req struct {
        Message string `json:"message"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    assistant := services.NewAIAssistant(database.Pool)
    response, err := assistant.ProcessRequest(tenantID, userID, req.Message)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"response": response})
}