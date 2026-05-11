package handlers

import (
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// ========== НОВЫЕ ФУНКЦИИ ДЛЯ ПЛАТФОРМЫ (ТОЛЬКО ДЛЯ ВЛАДЕЛЬЦА) ==========

// GetPlatformStaff - список помощников платформы
func GetPlatformStaff(c *gin.Context) {
    // TODO: реализовать получение списка platformAdmins и platformDevelopers
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "staff":   []gin.H{},
        "message": "Функция в разработке",
    })
}

// AddPlatformAdmin - добавить администратора платформы
func AddPlatformAdmin(c *gin.Context) {
    var req struct {
        Email string `json:"email" binding:"required"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // TODO: добавить email в platformAdmins в БД или конфиг
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Администратор платформы добавлен",
    })
}

// AddPlatformDeveloper - добавить разработчика платформы
func AddPlatformDeveloper(c *gin.Context) {
    var req struct {
        Email string `json:"email" binding:"required"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // TODO: добавить email в platformDevelopers в БД или конфиг
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Разработчик платформы добавлен",
    })
}

// RemovePlatformStaff - удалить помощника платформы
func RemovePlatformStaff(c *gin.Context) {
    email := c.Param("email")
    
    // TODO: удалить email из списка помощников
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Помощник удалён: " + email,
    })
}

// GetPlatformSettings - получить настройки платформы
func GetPlatformSettings(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "settings": gin.H{
            "app_name":     "SaaSPro",
            "app_version":  "3.0",
            "company_name": "BusinessStack",
        },
    })
}

// UpdatePlatformSettings - обновить настройки платформы
func UpdatePlatformSettings(c *gin.Context) {
    var req struct {
        AppName     string `json:"app_name"`
        CompanyName string `json:"company_name"`
    }
    c.BindJSON(&req)

    // TODO: сохранить настройки в БД или конфиг
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки обновлены",
    })
}

// SetTenantAdmin - назначить админа организации
func SetTenantAdmin(c *gin.Context) {
    var req struct {
        UserID   string `json:"user_id" binding:"required"`
        TenantID string `json:"tenant_id" binding:"required"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE users SET role = 'admin' WHERE id = $1 AND tenant_id = $2
    `, req.UserID, req.TenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Администратор назначен",
    })
}

// SetTenantDeveloper - назначить разработчика организации
func SetTenantDeveloper(c *gin.Context) {
    var req struct {
        UserID   string `json:"user_id" binding:"required"`
        TenantID string `json:"tenant_id" binding:"required"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE users SET role = 'developer' WHERE id = $1 AND tenant_id = $2
    `, req.UserID, req.TenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Разработчик назначен",
    })
}

// GrantModuleAccess - выдать доступ к модулю пользователю
func GrantModuleAccess(c *gin.Context) {
    var req struct {
        UserID     string `json:"user_id" binding:"required"`
        ModuleName string `json:"module_name" binding:"required"`
        Days       int    `json:"days"`
    }

    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.Days == 0 {
        req.Days = 14
    }

    expiresAt := time.Now().Add(time.Duration(req.Days) * 24 * time.Hour)

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO user_subscriptions (user_id, module_name, status, expires_at, created_at)
        VALUES ($1, $2, 'active', $3, NOW())
        ON CONFLICT (user_id, module_name) DO UPDATE SET
            status = 'active',
            expires_at = $3,
            updated_at = NOW()
    `, req.UserID, req.ModuleName, expiresAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Доступ к модулю выдан",
    })
}

// ========== АДМИНКА ОРГАНИЗАЦИИ (ЗАГЛУШКИ ДЛЯ КЛИЕНТОВ) ==========

func TenantAdminDashboard(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Админ-панель организации",
    })
}

func TenantGetUsers(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "users":   []gin.H{},
    })
}

func TenantCreateUser(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Функция в разработке",
    })
}

func TenantSetRole(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Функция в разработке",
    })
}

func TenantDeleteUser(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Функция в разработке",
    })
}

func TenantGetModules(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "modules": []gin.H{},
    })
}

func TenantGrantModuleAccess(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Функция в разработке",
    })
}