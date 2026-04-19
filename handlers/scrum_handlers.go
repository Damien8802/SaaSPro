package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "subscription-system/database"
)

// Sprint - структура спринта
type Sprint struct {
    ID             string    `json:"id"`
    Name           string    `json:"name"`
    Goal           string    `json:"goal"`
    StartDate      time.Time `json:"start_date"`
    EndDate        time.Time `json:"end_date"`
    Status         string    `json:"status"`
    TotalPoints    int       `json:"total_points"`
    CompletedPoints int      `json:"completed_points"`
}

// ScrumTask - структура задачи
type ScrumTask struct {
    ID           string `json:"id"`
    SprintID     string `json:"sprint_id"`
    Title        string `json:"title"`
    Description  string `json:"description"`
    Status       string `json:"status"`
    Priority     string `json:"priority"`
    StoryPoints  int    `json:"story_points"`
    AssigneeID   string `json:"assignee_id"`
    AssigneeName string `json:"assignee_name"`
}

// CreateSprint - создание спринта
func CreateSprint(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name      string    `json:"name" binding:"required"`
        Goal      string    `json:"goal"`
        StartDate time.Time `json:"start_date" binding:"required"`
        EndDate   time.Time `json:"end_date" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    sprintID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO sprints (id, tenant_id, name, goal, start_date, end_date, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, 'planning', NOW())
    `, sprintID, tenantID, req.Name, req.Goal, req.StartDate, req.EndDate)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":   "Спринт создан",
        "sprint_id": sprintID,
    })
}

// GetSprints - список спринтов
func GetSprints(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT s.id, s.name, COALESCE(s.goal, ''), s.start_date, s.end_date, s.status,
               COALESCE(SUM(t.story_points), 0) as total_points,
               COALESCE(SUM(CASE WHEN t.status = 'done' THEN t.story_points ELSE 0 END), 0) as completed_points
        FROM sprints s
        LEFT JOIN scrum_tasks t ON s.id = t.sprint_id
        WHERE s.tenant_id = $1
        GROUP BY s.id
        ORDER BY s.created_at DESC
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var sprints []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, goal, status string
        var startDate, endDate time.Time
        var totalPoints, completedPoints int

        rows.Scan(&id, &name, &goal, &startDate, &endDate, &status, &totalPoints, &completedPoints)

        sprints = append(sprints, gin.H{
            "id":               id.String(),
            "name":             name,
            "goal":             goal,
            "start_date":       startDate,
            "end_date":         endDate,
            "status":           status,
            "total_points":     totalPoints,
            "completed_points": completedPoints,
        })
    }

    c.JSON(http.StatusOK, sprints)
}

// StartSprint - начать спринт
func StartSprint(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    sprintID := c.Param("id")

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE sprints SET status = 'active', start_date = NOW()
        WHERE id = $1 AND tenant_id = $2
    `, sprintID, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Спринт начат"})
}

// CompleteSprint - завершить спринт
func CompleteSprint(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    sprintID := c.Param("id")

    var totalPoints, completedPoints int
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(story_points), 0), COALESCE(SUM(CASE WHEN status = 'done' THEN story_points ELSE 0 END), 0)
        FROM scrum_tasks WHERE sprint_id = $1
    `, sprintID).Scan(&totalPoints, &completedPoints)

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE sprints SET status = 'completed', total_points = $1, completed_points = $2, end_date = NOW()
        WHERE id = $3 AND tenant_id = $4
    `, totalPoints, completedPoints, sprintID, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":          "Спринт завершён",
        "total_points":     totalPoints,
        "completed_points": completedPoints,
    })
}

// GetScrumBoard - получить доску задач
func GetScrumBoard(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    sprintID := c.Param("sprint_id")

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT t.id, t.title, COALESCE(t.description, ''), t.status, t.priority, t.story_points,
               COALESCE(t.assignee_id::text, ''), COALESCE(e.full_name, 'Unassigned')
        FROM scrum_tasks t
        LEFT JOIN hr_employees e ON t.assignee_id = e.id
        WHERE t.sprint_id = $1 AND t.tenant_id = $2
        ORDER BY t.column_order
    `, sprintID, tenantID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "todo": []gin.H{}, "in_progress": []gin.H{}, "review": []gin.H{}, "done": []gin.H{},
        })
        return
    }
    defer rows.Close()

    board := gin.H{
        "todo":        []gin.H{},
        "in_progress": []gin.H{},
        "review":      []gin.H{},
        "done":        []gin.H{},
    }

    for rows.Next() {
        var id uuid.UUID
        var title, description, status, priority, assigneeID, assigneeName string
        var storyPoints int

        rows.Scan(&id, &title, &description, &status, &priority, &storyPoints, &assigneeID, &assigneeName)

        task := gin.H{
            "id":            id.String(),
            "title":         title,
            "description":   description,
            "priority":      priority,
            "story_points":  storyPoints,
            "assignee_name": assigneeName,
        }

        board[status] = append(board[status].([]gin.H), task)
    }

    c.JSON(http.StatusOK, board)
}

// CreateScrumTask - создание задачи
func CreateScrumTask(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        SprintID    string `json:"sprint_id" binding:"required"`
        Title       string `json:"title" binding:"required"`
        Description string `json:"description"`
        StoryPoints int    `json:"story_points"`
        Priority    string `json:"priority"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.Priority == "" {
        req.Priority = "medium"
    }
    if req.StoryPoints == 0 {
        req.StoryPoints = 1
    }

    taskID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO scrum_tasks (id, tenant_id, sprint_id, title, description, status, priority, story_points, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, 'todo', $6, $7, NOW(), NOW())
    `, taskID, tenantID, req.SprintID, req.Title, req.Description, req.Priority, req.StoryPoints)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Задача создана",
        "task_id": taskID,
    })
}

// UpdateTaskStatus - обновление статуса задачи
func UpdateTaskStatus(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    taskID := c.Param("id")

    var req struct {
        Status string `json:"status" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE scrum_tasks SET status = $1, updated_at = NOW()
        WHERE id = $2 AND tenant_id = $3
    `, req.Status, taskID, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ReorderTasks - переупорядочивание задач
func ReorderTasks(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        SprintID string   `json:"sprint_id" binding:"required"`
        Status   string   `json:"status" binding:"required"`
        TaskIDs  []string `json:"task_ids" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    for i, taskID := range req.TaskIDs {
        database.Pool.Exec(c.Request.Context(), `
            UPDATE scrum_tasks SET column_order = $1
            WHERE id = $2 AND sprint_id = $3 AND tenant_id = $4
        `, i, taskID, req.SprintID, tenantID)
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// AddTaskComment - добавить комментарий
func AddTaskComment(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    taskID := c.Param("id")
    userID := c.GetString("user_id")
    userName := c.GetString("user_name")

    var req struct {
        Comment string `json:"comment" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO task_comments (id, tenant_id, task_id, user_id, user_name, comment, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `, uuid.New(), tenantID, taskID, userID, userName, req.Comment)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Комментарий добавлен"})
}

// GetTaskComments - получить комментарии
func GetTaskComments(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    taskID := c.Param("id")

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT user_name, comment, created_at
        FROM task_comments
        WHERE task_id = $1 AND tenant_id = $2
        ORDER BY created_at ASC
    `, taskID, tenantID)

    if err != nil {
        c.JSON(http.StatusOK, []gin.H{})
        return
    }
    defer rows.Close()

    var comments []gin.H
    for rows.Next() {
        var userName, comment string
        var createdAt time.Time
        rows.Scan(&userName, &comment, &createdAt)
        comments = append(comments, gin.H{
            "user":       userName,
            "comment":    comment,
            "created_at": createdAt.Format("2006-01-02 15:04"),
        })
    }

    c.JSON(http.StatusOK, comments)
}

// GetBurndownChart - данные для графика
func GetBurndownChart(c *gin.Context) {
    //sprintID := c.Param("sprint_id")
    c.JSON(http.StatusOK, gin.H{
        "burndown": []gin.H{},
        "total_points": 0,
        "days": 0,
    })
}

// GetVelocityChart - скорость команды
func GetVelocityChart(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT name, total_points, completed_points,
               CASE WHEN total_points > 0 
                    THEN (completed_points::float / total_points::float * 100)
                    ELSE 0 END as velocity
        FROM sprints
        WHERE tenant_id = $1 AND status = 'completed'
        ORDER BY end_date DESC
        LIMIT 6
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusOK, []gin.H{})
        return
    }
    defer rows.Close()

    var velocities []gin.H
    for rows.Next() {
        var name string
        var total, completed int
        var velocity float64
        rows.Scan(&name, &total, &completed, &velocity)
        velocities = append(velocities, gin.H{
            "sprint":    name,
            "total":     total,
            "completed": completed,
            "velocity":  velocity,
        })
    }

    c.JSON(http.StatusOK, velocities)
}

// CreateScrumTables - создание таблиц для скрама
func CreateScrumTables(c *gin.Context) {
    // Таблица спринтов
    _, err := database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS sprints (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            name VARCHAR(255) NOT NULL,
            goal TEXT,
            start_date DATE,
            end_date DATE,
            status VARCHAR(50) DEFAULT 'planning',
            total_points INTEGER DEFAULT 0,
            completed_points INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Таблица задач
    _, err = database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS scrum_tasks (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            sprint_id UUID NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
            title VARCHAR(500) NOT NULL,
            description TEXT,
            status VARCHAR(50) DEFAULT 'todo',
            priority VARCHAR(20) DEFAULT 'medium',
            story_points INTEGER DEFAULT 1,
            assignee_id UUID,
            column_order INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Таблица комментариев
    _, err = database.Pool.Exec(c.Request.Context(), `
        CREATE TABLE IF NOT EXISTS task_comments (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            tenant_id UUID NOT NULL,
            task_id UUID NOT NULL REFERENCES scrum_tasks(id) ON DELETE CASCADE,
            user_id UUID,
            user_name VARCHAR(255),
            comment TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Таблицы скрама созданы",
        "tables":  []string{"sprints", "scrum_tasks", "task_comments"},
    })
}