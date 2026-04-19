package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "subscription-system/database"
)

// ImportBankStatement - импорт банковской выписки
func ImportBankStatement(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не загружен"})
        return
    }

    importID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO import_logs (id, tenant_id, file_name, file_type, status, created_at)
        VALUES ($1, $2, $3, 'bank_statement', 'processing', NOW())
    `, importID, tenantID, file.Filename)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":   "Импорт банковской выписки запущен",
        "import_id": importID,
        "filename":  file.Filename,
        "status":    "processing",
    })
}

// ImportInvoices - импорт счетов от поставщиков
func ImportInvoices(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не загружен"})
        return
    }

    importID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO import_logs (id, tenant_id, file_name, file_type, status, created_at)
        VALUES ($1, $2, $3, 'invoices', 'processing', NOW())
    `, importID, tenantID, file.Filename)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":   "Импорт счетов запущен",
        "import_id": importID,
        "filename":  file.Filename,
    })
}

// ImportActs - импорт актов выполненных работ
func ImportActs(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не загружен"})
        return
    }

    importID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO import_logs (id, tenant_id, file_name, file_type, status, created_at)
        VALUES ($1, $2, $3, 'acts', 'processing', NOW())
    `, importID, tenantID, file.Filename)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":   "Импорт актов запущен",
        "import_id": importID,
        "filename":  file.Filename,
    })
}

// GetImportTemplates - получить шаблоны для импорта
func GetImportTemplates(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "templates": gin.H{
            "sberbank": gin.H{
                "name":        "Выписка Сбербанк",
                "description": "Импорт выписок из Сбербанк Бизнес",
                "columns":     []string{"Дата", "Описание", "Дебет", "Кредит", "Остаток"},
                "format":      "xlsx",
            },
            "tinkoff": gin.H{
                "name":        "Выписка Тинькофф",
                "description": "Импорт выписок из Тинькофф Бизнес",
                "columns":     []string{"Дата", "Описание", "Сумма", "Остаток", "Контрагент"},
                "format":      "xlsx",
            },
            "invoices": gin.H{
                "name":        "Счета от поставщиков",
                "description": "Импорт счетов от поставщиков",
                "columns":     []string{"Номер", "Поставщик", "ИНН", "Описание", "Дата", "Сумма", "НДС"},
                "format":      "xlsx",
            },
            "acts": gin.H{
                "name":        "Акты выполненных работ",
                "description": "Импорт актов выполненных работ",
                "columns":     []string{"Номер", "Дата", "Клиент", "Сумма"},
                "format":      "xlsx",
            },
        },
    })
}

// CreateImportTables - создание таблиц для импорта
func CreateImportTables(c *gin.Context) {
    _, err := database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS import_logs (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            file_name VARCHAR(255),
            file_type VARCHAR(50),
            status VARCHAR(50) DEFAULT 'pending',
            total_rows INTEGER DEFAULT 0,
            imported_rows INTEGER DEFAULT 0,
            error_message TEXT,
            created_at TIMESTAMP DEFAULT NOW(),
            completed_at TIMESTAMP
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS imported_data (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            import_id UUID NOT NULL,
            data_type VARCHAR(50),
            raw_data JSONB,
            status VARCHAR(50) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Таблицы для импорта созданы",
        "tables":  []string{"import_logs", "imported_data"},
    })
}