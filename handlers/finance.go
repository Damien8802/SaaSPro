package handlers

import (
    "database/sql"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// ChartOfAccount структура счета
type ChartOfAccount struct {
    ID          uuid.UUID  `json:"id"`
    Code        string     `json:"code"`
    Name        string     `json:"name"`
    AccountType string     `json:"account_type"`
    ParentID    *uuid.UUID `json:"parent_id"`
    Level       int        `json:"level"`
    IsGroup     bool       `json:"is_group"`
    Currency    string     `json:"currency"`
    Description string     `json:"description"`
    IsActive    bool       `json:"is_active"`
    CreatedAt   time.Time  `json:"created_at"`
}

// GetChartOfAccounts - получить план счетов
func GetChartOfAccounts(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, code, name, account_type, parent_id, level, is_group, currency, description, is_active, created_at
        FROM chart_of_accounts
        WHERE user_id = $1 AND is_active = true
        ORDER BY code
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var accounts []ChartOfAccount
    for rows.Next() {
        var a ChartOfAccount
        var parentID sql.NullString
        err := rows.Scan(
            &a.ID, &a.Code, &a.Name, &a.AccountType, &parentID,
            &a.Level, &a.IsGroup, &a.Currency, &a.Description,
            &a.IsActive, &a.CreatedAt,
        )
        if err != nil {
            continue
        }
        if parentID.Valid {
            id, _ := uuid.Parse(parentID.String)
            a.ParentID = &id
        }
        accounts = append(accounts, a)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "accounts": accounts,
    })
}

// CreateChartOfAccount - создать счет
func CreateChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Code        string     `json:"code" binding:"required"`
        Name        string     `json:"name" binding:"required"`
        AccountType string     `json:"account_type" binding:"required"`
        ParentID    *uuid.UUID `json:"parent_id"`
        Level       int        `json:"level"`
        IsGroup     bool       `json:"is_group"`
        Currency    string     `json:"currency"`
        Description string     `json:"description"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.Currency == "" {
        req.Currency = "RUB"
    }
    
    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO chart_of_accounts (user_id, code, name, account_type, parent_id, level, is_group, currency, description, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, NOW(), NOW())
        RETURNING id
    `, userID, req.Code, req.Name, req.AccountType, req.ParentID, req.Level, req.IsGroup, req.Currency, req.Description).Scan(&id)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "id":      id,
        "message": "Счет создан",
    })
}

// UpdateChartOfAccount - обновить счет
func UpdateChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    accountID := c.Param("id")
    
    var req struct {
        Code        string     `json:"code"`
        Name        string     `json:"name"`
        AccountType string     `json:"account_type"`
        ParentID    *uuid.UUID `json:"parent_id"`
        Level       int        `json:"level"`
        IsGroup     bool       `json:"is_group"`
        Currency    string     `json:"currency"`
        Description string     `json:"description"`
        IsActive    bool       `json:"is_active"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE chart_of_accounts SET
            code = $1, name = $2, account_type = $3, parent_id = $4,
            level = $5, is_group = $6, currency = $7, description = $8,
            is_active = $9, updated_at = NOW()
        WHERE id = $10 AND user_id = $11
    `, req.Code, req.Name, req.AccountType, req.ParentID,
        req.Level, req.IsGroup, req.Currency, req.Description,
        req.IsActive, accountID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Счет обновлен",
    })
}

// DeleteChartOfAccount - удалить счет
func DeleteChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    accountID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE chart_of_accounts SET is_active = false, updated_at = NOW()
        WHERE id = $1 AND user_id = $2
    `, accountID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Счет удален",
    })
}

// ==================== ЖУРНАЛ ПРОВОДОК ====================

// JournalEntry структура журнала проводок
type JournalEntry struct {
    ID          uuid.UUID  `json:"id"`
    EntryNumber string     `json:"entry_number"`
    EntryDate   time.Time  `json:"entry_date"`
    Description string     `json:"description"`
    SourceType  string     `json:"source_type"`
    SourceID    *uuid.UUID `json:"source_id"`
    TotalAmount float64    `json:"total_amount"`
    Status      string     `json:"status"`
    PostedBy    *uuid.UUID `json:"posted_by"`
    PostedAt    *time.Time `json:"posted_at"`
    Notes       string     `json:"notes"`
    CreatedAt   time.Time  `json:"created_at"`
}

// JournalPosting структура проводки
type JournalPosting struct {
    ID           uuid.UUID `json:"id"`
    EntryID      uuid.UUID `json:"entry_id"`
    AccountID    uuid.UUID `json:"account_id"`
    AccountCode  string    `json:"account_code"`
    AccountName  string    `json:"account_name"`
    DebitAmount  float64   `json:"debit_amount"`
    CreditAmount float64   `json:"credit_amount"`
    Description  string    `json:"description"`
    CreatedAt    time.Time `json:"created_at"`
}

// GetJournalEntries - получить журнал проводок
func GetJournalEntries(c *gin.Context) {
    userID := getUserID(c)
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    status := c.Query("status")
    
    query := `
        SELECT id, entry_number, entry_date, description, source_type, source_id,
               total_amount, entry_status, posted_by, posted_at, notes, created_at
        FROM journal_entries
        WHERE user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if startDate != "" {
        query += fmt.Sprintf(" AND entry_date >= $%d", argIndex)
        args = append(args, startDate)
        argIndex++
    }
    if endDate != "" {
        query += fmt.Sprintf(" AND entry_date <= $%d", argIndex)
        args = append(args, endDate)
        argIndex++
    }
    if status != "" {
        query += fmt.Sprintf(" AND entry_status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }
    
    query += " ORDER BY entry_date DESC, created_at DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var entries []JournalEntry
    for rows.Next() {
        var e JournalEntry
        var sourceID sql.NullString
        var postedBy sql.NullString
        var postedAt sql.NullTime
        
        err := rows.Scan(
            &e.ID, &e.EntryNumber, &e.EntryDate, &e.Description,
            &e.SourceType, &sourceID, &e.TotalAmount, &e.Status,
            &postedBy, &postedAt, &e.Notes, &e.CreatedAt,
        )
        if err != nil {
            continue
        }
        if sourceID.Valid {
            id, _ := uuid.Parse(sourceID.String)
            e.SourceID = &id
        }
        if postedBy.Valid {
            id, _ := uuid.Parse(postedBy.String)
            e.PostedBy = &id
        }
        if postedAt.Valid {
            e.PostedAt = &postedAt.Time
        }
        entries = append(entries, e)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "entries": entries,
    })
}

// GetJournalEntry - получить проводку по ID
func GetJournalEntry(c *gin.Context) {
    userID := getUserID(c)
    entryID := c.Param("id")
    
    var e JournalEntry
    var sourceID sql.NullString
    var postedBy sql.NullString
    var postedAt sql.NullTime
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, entry_number, entry_date, description, source_type, source_id,
               total_amount, entry_status, posted_by, posted_at, notes, created_at
        FROM journal_entries
        WHERE id = $1 AND user_id = $2
    `, entryID, userID).Scan(
        &e.ID, &e.EntryNumber, &e.EntryDate, &e.Description,
        &e.SourceType, &sourceID, &e.TotalAmount, &e.Status,
        &postedBy, &postedAt, &e.Notes, &e.CreatedAt,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Проводка не найдена"})
        return
    }
    
    if sourceID.Valid {
        id, _ := uuid.Parse(sourceID.String)
        e.SourceID = &id
    }
    if postedBy.Valid {
        id, _ := uuid.Parse(postedBy.String)
        e.PostedBy = &id
    }
    if postedAt.Valid {
        e.PostedAt = &postedAt.Time
    }
    
    // Получаем проводки
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, p.entry_id, p.account_id, a.code, a.name,
               p.debit_amount, p.credit_amount, p.description, p.created_at
        FROM journal_postings p
        JOIN chart_of_accounts a ON p.account_id = a.id
        WHERE p.entry_id = $1
    `, entryID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки проводок"})
        return
    }
    defer rows.Close()
    
    var postings []JournalPosting
    for rows.Next() {
        var p JournalPosting
        err := rows.Scan(
            &p.ID, &p.EntryID, &p.AccountID, &p.AccountCode,
            &p.AccountName, &p.DebitAmount, &p.CreditAmount,
            &p.Description, &p.CreatedAt,
        )
        if err != nil {
            continue
        }
        postings = append(postings, p)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "entry":    e,
        "postings": postings,
    })
}

// CreateJournalEntry - создать проводку
func CreateJournalEntry(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        EntryDate   string  `json:"entry_date"`
        Description string  `json:"description" binding:"required"`
        SourceType  string  `json:"source_type"`
        SourceID    string  `json:"source_id"`
        Notes       string  `json:"notes"`
        Postings    []struct {
            AccountID    string  `json:"account_id" binding:"required"`
            DebitAmount  float64 `json:"debit_amount"`
            CreditAmount float64 `json:"credit_amount"`
            Description  string  `json:"description"`
        } `json:"postings" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Проверяем баланс
    var totalDebit, totalCredit float64
    for _, p := range req.Postings {
        totalDebit += p.DebitAmount
        totalCredit += p.CreditAmount
    }
    
    if totalDebit != totalCredit {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Сумма дебета должна равняться сумме кредита",
        })
        return
    }
    
    // Генерируем номер
    entryNumber := fmt.Sprintf("ЖР-%d", time.Now().UnixNano()%1000000)
    
    entryDate := time.Now()
    if req.EntryDate != "" {
        ed, _ := time.Parse("2006-01-02", req.EntryDate)
        entryDate = ed
    }
    
    var sourceID *uuid.UUID
    if req.SourceID != "" {
        id, _ := uuid.Parse(req.SourceID)
        sourceID = &id
    }
    
    // Создаем в транзакции
    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка транзакции"})
        return
    }
    defer tx.Rollback(c.Request.Context())
    
    var entryID uuid.UUID
    err = tx.QueryRow(c.Request.Context(), `
        INSERT INTO journal_entries (
            user_id, entry_number, entry_date, description, source_type,
            source_id, total_amount, entry_status, notes, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, 'draft', $8, NOW(), NOW())
        RETURNING id
    `, userID, entryNumber, entryDate, req.Description,
        req.SourceType, sourceID, totalDebit, req.Notes).Scan(&entryID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать проводку"})
        return
    }
    
    // Добавляем проводки
    for _, p := range req.Postings {
        accountID, _ := uuid.Parse(p.AccountID)
        _, err = tx.Exec(c.Request.Context(), `
            INSERT INTO journal_postings (entry_id, account_id, debit_amount, credit_amount, description)
            VALUES ($1, $2, $3, $4, $5)
        `, entryID, accountID, p.DebitAmount, p.CreditAmount, p.Description)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось добавить проводки"})
            return
        }
    }
    
    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "entry_id":     entryID,
        "entry_number": entryNumber,
        "message":      "Проводка создана",
    })
}

// PostJournalEntry - провести проводку
func PostJournalEntry(c *gin.Context) {
    userID := getUserID(c)
    entryID := c.Param("id")
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE journal_entries 
        SET entry_status = 'posted', posted_by = $1, posted_at = NOW(), updated_at = NOW()
        WHERE id = $2 AND user_id = $3 AND entry_status = 'draft'
    `, userID, entryID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось провести проводку"})
        return
    }
    
    if result.RowsAffected() == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Проводка не найдена или уже проведена"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Проводка проведена",
    })
}

// DeleteJournalEntry - удалить проводку
func DeleteJournalEntry(c *gin.Context) {
    userID := getUserID(c)
    entryID := c.Param("id")
    
    // Проверяем статус
    var status string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT entry_status FROM journal_entries 
        WHERE id = $1 AND user_id = $2
    `, entryID, userID).Scan(&status)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Проводка не найдена"})
        return
    }
    
    if status == "posted" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя удалить проведенную проводку"})
        return
    }
    
    _, err = database.Pool.Exec(c.Request.Context(), `
        DELETE FROM journal_entries WHERE id = $1 AND user_id = $2
    `, entryID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Проводка удалена",
    })
}