package handlers

import (
    "net/http"
    "time"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// GetUserSessions - получение активных сессий пользователя
func GetUserSessions(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusOK, gin.H{"sessions": []interface{}{}})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, user_agent, ip, created_at, last_active, is_current
        FROM user_sessions
        WHERE user_id = $1 AND expires_at > NOW()
        ORDER BY last_active DESC
    `, userID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"sessions": []interface{}{}})
        return
    }
    defer rows.Close()

    var sessions []gin.H
    for rows.Next() {
        var id int
        var userAgent, ip string
        var createdAt, lastActive time.Time
        var isCurrent bool

        rows.Scan(&id, &userAgent, &ip, &createdAt, &lastActive, &isCurrent)

        sessions = append(sessions, gin.H{
            "id":          id,
            "browser":     parseBrowser(userAgent),
            "os":          parseOS(userAgent),
            "ip":          ip,
            "created_at":  createdAt,
            "last_active": lastActive,
            "is_current":  isCurrent,
        })
    }

    c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// TerminateSession - завершение конкретной сессии
func TerminateSession(c *gin.Context) {
    sessionID := c.Param("id")
    userID := c.GetString("user_id")

    _, err := database.Pool.Exec(c.Request.Context(),
        "DELETE FROM user_sessions WHERE id = $1 AND user_id = $2",
        sessionID, userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// TerminateAllSessions - завершение всех сессий кроме текущей
func TerminateAllSessions(c *gin.Context) {
    userID := c.GetString("user_id")

    _, err := database.Pool.Exec(c.Request.Context(),
        "DELETE FROM user_sessions WHERE user_id = $1 AND is_current = false",
        userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetLoginHistory - история входов пользователя
func GetLoginHistory(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusOK, gin.H{"history": []interface{}{}})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, ip, user_agent, success, created_at
        FROM login_history
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"history": []interface{}{}})
        return
    }
    defer rows.Close()

    var history []gin.H
    for rows.Next() {
        var id int
        var ip, userAgent string
        var success bool
        var createdAt time.Time

        rows.Scan(&id, &ip, &userAgent, &success, &createdAt)

        history = append(history, gin.H{
            "id":         id,
            "ip":         ip,
            "browser":    parseBrowser(userAgent),
            "os":         parseOS(userAgent),
            "success":    success,
            "created_at": createdAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{"history": history})
}

// GetUserSettings - настройки пользователя
func GetUserSettings(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusOK, gin.H{"max_sessions": 5})
        return
    }

    var maxSessions int
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT COALESCE(max_sessions, 5) FROM users WHERE id = $1", userID).Scan(&maxSessions)
    if err != nil {
        maxSessions = 5
    }

    c.JSON(http.StatusOK, gin.H{"max_sessions": maxSessions})
}

// SetMaxSessions - установка лимита сессий
func SetMaxSessions(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }

    var req struct {
        MaxSessions int `json:"max_sessions"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(),
        "UPDATE users SET max_sessions = $1 WHERE id = $2",
        req.MaxSessions, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// Вспомогательные функции (без конфликта с contains)
func parseBrowser(userAgent string) string {
    if stringContains(userAgent, "Chrome") && !stringContains(userAgent, "Edg") {
        return "Chrome"
    }
    if stringContains(userAgent, "Firefox") {
        return "Firefox"
    }
    if stringContains(userAgent, "Safari") && !stringContains(userAgent, "Chrome") {
        return "Safari"
    }
    if stringContains(userAgent, "Edg") {
        return "Edge"
    }
    return "Другой"
}

func parseOS(userAgent string) string {
    if stringContains(userAgent, "Windows") {
        return "Windows"
    }
    if stringContains(userAgent, "Mac") {
        return "macOS"
    }
    if stringContains(userAgent, "Linux") {
        return "Linux"
    }
    if stringContains(userAgent, "Android") {
        return "Android"
    }
    if stringContains(userAgent, "iPhone") || stringContains(userAgent, "iPad") {
        return "iOS"
    }
    return "Другая"
}

func stringContains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr))
}
