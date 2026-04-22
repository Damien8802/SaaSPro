package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

func AIAskHandler(c *gin.Context) {
    var req struct {
        Question string `json:"question"`
        Context  string `json:"context"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Временный ответ
    answer := "Функция AIAskHandler временно недоступна. Используйте AI ассистента в чате."
    
    c.JSON(http.StatusOK, gin.H{
        "answer": answer,
    })
}