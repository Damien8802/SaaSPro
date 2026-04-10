package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// NebulaCloudPage - страница облачного хранилища
func NebulaCloudPage(c *gin.Context) {
    c.HTML(http.StatusOK, "cloud.html", gin.H{
        "title": "Nebula Cloud — Облачное хранилище",
        "brand": "Nebula Cloud",
    })
}

// GetCloudFiles - получить список файлов
func GetCloudFiles(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    folder := c.DefaultQuery("folder", "/")
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, size, mime_type, folder, is_starred, is_shared, created_at
        FROM cloud_files 
        WHERE user_id = $1 AND folder = $2 AND is_active = true
        ORDER BY is_starred DESC, created_at DESC
    `, userID, folder)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load files"})
        return
    }
    defer rows.Close()
    
    var files []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, mimeType, folder string
        var size int64
        var isStarred, isShared bool
        var createdAt time.Time
        
        rows.Scan(&id, &name, &size, &mimeType, &folder, &isStarred, &isShared, &createdAt)
        
        files = append(files, gin.H{
            "id":         id,
            "name":       name,
            "size":       size,
            "size_mb":    float64(size) / 1024 / 1024,
            "mime_type":  mimeType,
            "is_starred": isStarred,
            "is_shared":  isShared,
            "created_at": createdAt,
            "icon":       getFileIcon(mimeType, name),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"files": files})
}

// UploadCloudFile - загрузить файл
func UploadCloudFile(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
        return
    }
    
    folder := c.DefaultPostForm("folder", "/")
    
    // Создаём директорию
    uploadDir := fmt.Sprintf("./cloud_storage/%s/%s", userID, folder)
    if err := os.MkdirAll(uploadDir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
        return
    }
    
    // Сохраняем файл
    filename := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)
    filePath := filepath.Join(uploadDir, filename)
    
    if err := c.SaveUploadedFile(file, filePath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
        return
    }
    
    // Сохраняем в БД
    fileID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO cloud_files (id, user_id, name, path, size, mime_type, folder, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
    `, fileID, userID, file.Filename, filePath, file.Size, file.Header.Get("Content-Type"), folder)
    
    if err != nil {
        log.Printf("❌ Ошибка сохранения: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "id":      fileID,
        "name":    file.Filename,
        "size":    file.Size,
        "size_mb": float64(file.Size) / 1024 / 1024,
        "message": "Файл загружен",
    })
}

// DeleteCloudFile - удалить файл
func DeleteCloudFile(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    fileID := c.Param("id")
    
    // Получаем информацию о файле
    var filePath string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT path FROM cloud_files WHERE id = $1 AND user_id = $2
    `, fileID, userID).Scan(&filePath)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
        return
    }
    
    // Удаляем файл
    if err := os.Remove(filePath); err != nil {
        log.Printf("⚠️ Не удалось удалить файл: %v", err)
    }
    
    // Удаляем из БД
    _, err = database.Pool.Exec(c.Request.Context(), `
        DELETE FROM cloud_files WHERE id = $1 AND user_id = $2
    `, fileID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Файл удалён"})
}

// CreateCloudFolder - создать папку
func CreateCloudFolder(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        Name   string `json:"name" binding:"required"`
        Parent string `json:"parent"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    folderPath := req.Parent
    if folderPath == "" {
        folderPath = "/"
    }
    
    // Создаём папку на диске
    dirPath := fmt.Sprintf("./cloud_storage/%s/%s/%s", userID, folderPath, req.Name)
    if err := os.MkdirAll(dirPath, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create folder"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "name":    req.Name,
        "path":    folderPath + req.Name + "/",
        "message": "Папка создана",
    })
}

// GetCloudStats - получить статистику использования
func GetCloudStats(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    // Проверяем есть ли бакет у пользователя
    var bucketID uuid.UUID
    var planName string
    var quotaGB int
    var pricePaid float64
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT b.id, p.name, p.quota_gb, b.price_paid
        FROM cloud_buckets b
        JOIN cloud_plans p ON b.plan_id = p.id
        WHERE b.user_id = $1 AND b.is_active = true
        LIMIT 1
    `, userID).Scan(&bucketID, &planName, &quotaGB, &pricePaid)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "has_storage": false,
            "message":     "У вас нет облачного хранилища",
        })
        return
    }
    
    // Считаем использованное место из файлов
    var totalSize int64
    var filesCount int
    
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(size), 0), COUNT(*)
        FROM cloud_files WHERE user_id = $1
    `, userID).Scan(&totalSize, &filesCount)
    
    usedGB := float64(totalSize) / 1024 / 1024 / 1024
    usedPercent := (usedGB / float64(quotaGB)) * 100
    if usedPercent > 100 {
        usedPercent = 100
    }
    
    c.JSON(http.StatusOK, gin.H{
        "has_storage":   true,
        "used_gb":       roundFloat(usedGB, 2),
        "total_gb":      quotaGB,
        "used_percent":  roundFloat(usedPercent, 2),
        "files_count":   filesCount,
        "plan_name":     planName,
        "price_paid":    pricePaid,
        "is_free":       pricePaid == 0,
    })
}

// GetCloudPlans - получить тарифы (безлимитный показываем только разработчикам)
func GetCloudPlans(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    // Проверяем, разработчик ли это
    isDev := isUserDeveloper(c, userID)
    
    var query string
    if isDev {
        // Разработчикам показываем все тарифы
        query = `SELECT id, name, quota_gb, price_monthly, price_yearly, is_free_for_dev, sort_order 
                 FROM cloud_plans WHERE is_active = true ORDER BY sort_order`
    } else {
        // Обычным пользователям скрываем тариф Nebula Dev
        query = `SELECT id, name, quota_gb, price_monthly, price_yearly, is_free_for_dev, sort_order 
                 FROM cloud_plans WHERE is_active = true AND name != 'Nebula Dev' ORDER BY sort_order`
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), query)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load plans"})
        return
    }
    defer rows.Close()
    
    var plans []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name string
        var quotaGB int
        var priceMonthly, priceYearly float64
        var isFreeForDev bool
        var sortOrder int
        
        rows.Scan(&id, &name, &quotaGB, &priceMonthly, &priceYearly, &isFreeForDev, &sortOrder)
        
        plans = append(plans, gin.H{
            "id":              id,
            "name":            name,
            "quota_gb":        quotaGB,
            "price_monthly":   priceMonthly,
            "price_yearly":    priceYearly,
            "is_free_for_dev": isFreeForDev,
        })
    }
    
    c.JSON(http.StatusOK, plans)
}
// CreateCloudBucket - создать бакет для клиента
func CreateCloudBucket(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        PlanID string `json:"plan_id" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Проверяем, разработчик ли это
    isDev := isUserDeveloper(c, userID)
    
    var planID uuid.UUID
    var quotaGB int
    var planName string
    var priceMonthly float64
    var isFreeForDev bool
    
    // Если не разработчик - проверяем что выбран не Dev тариф
    if !isDev {
        var planNameCheck string
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT name FROM cloud_plans WHERE id = $1
        `, req.PlanID).Scan(&planNameCheck)
        
        if planNameCheck == "Nebula Dev" {
            c.JSON(http.StatusForbidden, gin.H{"error": "This plan is not available for regular users"})
            return
        }
    }
    
    // Получаем тариф
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, name, quota_gb, price_monthly, is_free_for_dev
        FROM cloud_plans WHERE id = $1 AND is_active = true
    `, req.PlanID).Scan(&planID, &planName, &quotaGB, &priceMonthly, &isFreeForDev)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
        return
    }
    
    var finalPrice float64
    
    if isDev && isFreeForDev {
        finalPrice = 0
    } else {
        finalPrice = priceMonthly
    }
    
    // Генерируем имя бакета
    bucketName := fmt.Sprintf("nebula-%s-%s", userID[:8], uuid.New().String()[:8])
    
    // Генерируем ключи
    accessKey := generateAccessKey()
    secretKey := generateSecretKey()
    
    // Создаём директорию
    dirPath := fmt.Sprintf("./cloud_storage/%s", userID)
    if err := os.MkdirAll(dirPath, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
        return
    }
    
    // Сохраняем в БД
    bucketID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO cloud_buckets (id, user_id, plan_id, bucket_name, access_key, secret_key, price_paid, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
    `, bucketID, userID, planID, bucketName, accessKey, secretKey, finalPrice)
    
    if err != nil {
        log.Printf("❌ Ошибка создания бакета: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bucket"})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{
        "bucket_id":   bucketID,
        "bucket_name": bucketName,
        "access_key":  accessKey,
        "secret_key":  secretKey,
        "endpoint":    "http://localhost:9000",
        "quota_gb":    quotaGB,
        "price":       finalPrice,
        "is_free":     finalPrice == 0,
        "plan_name":   planName,
    })
}
// UpgradeCloudPlan - обновить тариф
func UpgradeCloudPlan(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        PlanID string `json:"plan_id" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Printf("❌ Ошибка парсинга JSON: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    log.Printf("🔄 UpgradeCloudPlan: userID=%s, planID=%s", userID, req.PlanID)
    
    // Получаем новый тариф
    var newPlanID uuid.UUID
    var newPlanName string
    var newQuotaGB int
    var newPriceMonthly float64
    var isFreeForDev bool
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, name, quota_gb, price_monthly, is_free_for_dev
        FROM cloud_plans WHERE id = $1 AND is_active = true
    `, req.PlanID).Scan(&newPlanID, &newPlanName, &newQuotaGB, &newPriceMonthly, &isFreeForDev)
    
    if err != nil {
        log.Printf("❌ Тариф не найден: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
        return
    }
    
    log.Printf("✅ Найден тариф: %s, квота: %d ГБ, цена: %.2f", newPlanName, newQuotaGB, newPriceMonthly)
    
    // Проверяем, разработчик ли это
    isDev := isUserDeveloper(c, userID)
    
    var newPrice float64
    if isDev && isFreeForDev {
        newPrice = 0
    } else {
        newPrice = newPriceMonthly
    }
    
    log.Printf("💰 Итоговая цена: %.2f (isDev=%v, isFreeForDev=%v)", newPrice, isDev, isFreeForDev)
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE cloud_buckets 
        SET plan_id = $1, price_paid = $2, updated_at = NOW()
        WHERE user_id = $3 AND is_active = true
    `, newPlanID, newPrice, userID)
    
    if err != nil {
        log.Printf("❌ Ошибка обновления БД: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade plan"})
        return
    }
    
    rowsAffected := result.RowsAffected()
    log.Printf("✅ Обновлено строк: %d", rowsAffected)
    
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "No active bucket found for this user"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":   "Тариф обновлён",
        "plan_name": newPlanName,
        "quota_gb":  newQuotaGB,
        "price":     newPrice,
        "is_free":   newPrice == 0,
    })
}
// getFileIcon - иконка файла
func getFileIcon(mimeType, filename string) string {
    if strings.Contains(mimeType, "image") {
        return "bi-file-image"
    }
    if strings.Contains(mimeType, "video") {
        return "bi-file-play"
    }
    if strings.Contains(mimeType, "audio") {
        return "bi-file-music"
    }
    if strings.Contains(mimeType, "pdf") {
        return "bi-file-pdf"
    }
    if strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "rar") {
        return "bi-file-zip"
    }
    if strings.Contains(mimeType, "word") || strings.Contains(filename, ".doc") {
        return "bi-file-word"
    }
    if strings.Contains(mimeType, "excel") || strings.Contains(filename, ".xls") {
        return "bi-file-excel"
    }
    return "bi-file-text"
}

// roundFloat - округление
func roundFloat(val float64, precision int) float64 {
    ratio := 1.0
    for i := 0; i < precision; i++ {
        ratio *= 10
    }
    return float64(int(val*ratio)) / ratio
}

// generateAccessKey - генерация access key
func generateAccessKey() string {
    bytes := make([]byte, 16)
    rand.Read(bytes)
    return "AKIA" + hex.EncodeToString(bytes)[:16]
}

// generateSecretKey - генерация secret key
func generateSecretKey() string {
    bytes := make([]byte, 32)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}

// isUserDeveloper - проверка, разработчик ли пользователь
func isUserDeveloper(c *gin.Context, userID string) bool {
    if c.GetHeader("X-Developer-Access") == "fusion-dev-2024" {
        return true
    }
    return false
}

// DownloadCloudFile - скачать файл
func DownloadCloudFile(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    fileID := c.Param("id")
    
    var filePath, fileName string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT path, name FROM cloud_files WHERE id = $1 AND user_id = $2
    `, fileID, userID).Scan(&filePath, &fileName)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
        return
    }
    
    c.FileAttachment(filePath, fileName)
}

// ToggleStarFile - добавить/убрать из избранного
func ToggleStarFile(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    fileID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE cloud_files SET is_starred = NOT is_starred
        WHERE id = $1 AND user_id = $2
    `, fileID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}
