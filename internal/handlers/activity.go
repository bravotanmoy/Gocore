package handlers

import (
	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListActivity handles GET /api/v1/activity?user_id=&action=&entity=&search=
func (h *Handler) ListActivity(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.ActivityLog{}).Preload("User")

	if uid := c.Query("user_id"); uid != "" {
		q = q.Where("user_id = ?", uid)
	}
	if a := c.Query("action"); a != "" {
		q = q.Where("action = ?", a)
	}
	if e := c.Query("entity"); e != "" {
		q = q.Where("entity = ?", e)
	}
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("detail LIKE ? OR path LIKE ?", like, like)
	}

	var total int64
	q.Count(&total)
	var logs []models.ActivityLog
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&logs)
	utils.Paginated(c, logs, total, page, perPage)
}
