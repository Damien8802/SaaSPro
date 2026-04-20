package handlers

import (
    "fmt"
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// AdminGetPayouts - получить все заявки на вывод
func AdminGetPayouts(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, p.user_id, p.amount, p.method, p.status, p.created_at, 
               COALESCE(u.email, '') as user_email,
               COALESCE(d.details, '{}') as details
        FROM referral_payouts p
        LEFT JOIN users u ON p.user_id = u.id
        LEFT JOIN user_payout_details d ON d.user_id = p.user_id AND d.method = p.method
        ORDER BY p.created_at DESC
    `)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"payouts": []interface{}{}})
        return
    }
    defer rows.Close()
    
    var payouts []gin.H
    for rows.Next() {
        var id int
        var userID, method, status, createdAt, userEmail, details string
        var amount float64
        
        rows.Scan(&id, &userID, &amount, &method, &status, &createdAt, &userEmail, &details)
        
        payouts = append(payouts, gin.H{
            "id":         id,
            "user_id":    userID,
            "user_email": userEmail,
            "amount":     amount,
            "method":     method,
            "status":     status,
            "created_at": createdAt,
            "details":    details,
        })
    }
    
    if payouts == nil {
        payouts = []gin.H{}
    }
    
    c.JSON(http.StatusOK, gin.H{"payouts": payouts})
}

// AdminUpdatePayoutStatus - обновить статус выплаты
func AdminUpdatePayoutStatus(c *gin.Context) {
    var req struct {
        ID     int    `json:"id"`
        Status string `json:"status"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Получаем user_id и сумму перед обновлением для уведомления
    var userID string
    var amount float64
    var userEmail string
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT p.user_id, p.amount, COALESCE(u.email, '')
        FROM referral_payouts p
        LEFT JOIN users u ON p.user_id = u.id
        WHERE p.id = $1
    `, req.ID).Scan(&userID, &amount, &userEmail)
    
    // Обновляем статус
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE referral_payouts SET status = $1, updated_at = NOW()
        WHERE id = $2
    `, req.Status, req.ID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Если статус изменен на completed - отправляем уведомление
    if req.Status == "completed" {
        // Сохраняем уведомление в БД
        _, _ = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO notifications (user_id, title, message, type, is_read, created_at)
            VALUES ($1, $2, $3, 'payout', false, NOW())
        `, userID, "✅ Выплата выполнена", fmt.Sprintf("Сумма %.0f ₽ успешно переведена", amount))
        
        // Логируем действие
        fmt.Printf("📧 Уведомление отправлено пользователю %s: Выплата %.0f ₽ выполнена\n", userEmail, amount)
    }
    
    if req.Status == "rejected" {
        // Сохраняем уведомление об отказе
        _, _ = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO notifications (user_id, title, message, type, is_read, created_at)
            VALUES ($1, $2, $3, 'payout', false, NOW())
        `, userID, "❌ Выплата отклонена", fmt.Sprintf("Заявка на сумму %.0f ₽ отклонена", amount))
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true})
}