package handlers

import (
	"net/http"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// ---------- Settings ----------

// ListSettings handles GET /api/v1/settings — grouped map for the UI.
func (h *Handler) ListSettings(c *gin.Context) {
	var settings []models.Setting
	database.DB.Order("setting_key").Find(&settings)
	grouped := map[string][]models.Setting{}
	for _, s := range settings {
		grouped[s.Group] = append(grouped[s.Group], s)
	}
	utils.OK(c, grouped)
}

// UpdateSettings handles PUT /api/v1/settings {"settings": {"key": "value", ...}}
func (h *Handler) UpdateSettings(c *gin.Context) {
	var in struct {
		Settings map[string]string `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	for key, value := range in.Settings {
		var s models.Setting
		if database.DB.Where("setting_key = ?", key).First(&s).Error == nil {
			database.DB.Model(&s).Update("value", value)
		} else {
			database.DB.Create(&models.Setting{Key: key, Value: value, Group: "custom"})
		}
	}
	c.Set("activity", "updated site settings")
	c.Set("activity_entity", "setting")
	utils.OK(c, gin.H{"message": "settings saved"})
}

// ---------- Contact messages ----------

// ListContactMessages handles GET /api/v1/contact-messages
func (h *Handler) ListContactMessages(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.ContactMessage{})
	if c.Query("unread") == "true" {
		q = q.Where("is_read = false")
	}
	var total int64
	q.Count(&total)
	var msgs []models.ContactMessage
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&msgs)
	utils.Paginated(c, msgs, total, page, perPage)
}

// MarkContactRead handles PUT /api/v1/contact-messages/:id/read
func (h *Handler) MarkContactRead(c *gin.Context) {
	database.DB.Model(&models.ContactMessage{}).Where("id = ?", c.Param("id")).Update("is_read", true)
	utils.OK(c, gin.H{"message": "marked read"})
}

// DeleteContactMessage handles DELETE /api/v1/contact-messages/:id
func (h *Handler) DeleteContactMessage(c *gin.Context) {
	database.DB.Delete(&models.ContactMessage{}, c.Param("id"))
	c.Set("activity", "deleted contact message #"+c.Param("id"))
	utils.OK(c, gin.H{"message": "deleted"})
}

// ---------- Todos ----------

// ListTodos handles GET /api/v1/todos (current user's todos).
func (h *Handler) ListTodos(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var todos []models.Todo
	database.DB.Where("user_id = ?", user.ID).Order("done, id DESC").Find(&todos)
	utils.OK(c, todos)
}

// CreateTodo handles POST /api/v1/todos {title}
func (h *Handler) CreateTodo(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var in struct {
		Title string `json:"title" form:"title" binding:"required"`
	}
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	todo := models.Todo{UserID: user.ID, Title: in.Title}
	database.DB.Create(&todo)
	utils.Created(c, todo)
}

// ToggleTodo handles PUT /api/v1/todos/:id/toggle
func (h *Handler) ToggleTodo(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var todo models.Todo
	if err := database.DB.Where("user_id = ?", user.ID).First(&todo, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "todo not found")
		return
	}
	database.DB.Model(&todo).Update("done", !todo.Done)
	utils.OK(c, todo)
}

// DeleteTodo handles DELETE /api/v1/todos/:id
func (h *Handler) DeleteTodo(c *gin.Context) {
	user := middleware.CurrentUser(c)
	database.DB.Where("user_id = ?", user.ID).Delete(&models.Todo{}, c.Param("id"))
	utils.OK(c, gin.H{"message": "deleted"})
}

// ---------- Notifications ----------

// ListNotifications handles GET /api/v1/notifications (mine + broadcasts).
func (h *Handler) ListNotifications(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var items []models.Notification
	database.DB.Where("user_id = ? OR user_id IS NULL", user.ID).
		Order("id DESC").Limit(50).Find(&items)
	utils.OK(c, items)
}

// SendNotification handles POST /api/v1/notifications {title, body, type, user_id?}
// user_id omitted = broadcast. Also pushes over websocket in real time.
func (h *Handler) SendNotification(c *gin.Context) {
	var in struct {
		Title  string `json:"title" binding:"required"`
		Body   string `json:"body"`
		Type   string `json:"type"`
		UserID *uint  `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Type == "" {
		in.Type = "info"
	}
	n := models.Notification{UserID: in.UserID, Title: in.Title, Body: in.Body, Type: in.Type}
	database.DB.Create(&n)
	if in.UserID != nil {
		realtime.H.SendToUser(*in.UserID, "notification", n)
	} else {
		realtime.H.Broadcast("notification", n)
	}
	c.Set("activity", "sent notification \""+in.Title+"\"")
	utils.Created(c, n)
}

// MarkNotificationRead handles PUT /api/v1/notifications/:id/read
func (h *Handler) MarkNotificationRead(c *gin.Context) {
	database.DB.Model(&models.Notification{}).Where("id = ?", c.Param("id")).Update("is_read", true)
	utils.OK(c, gin.H{"message": "marked read"})
}
