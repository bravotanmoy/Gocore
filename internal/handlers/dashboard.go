package handlers

import (
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// Stats handles GET /api/v1/dashboard/stats — headline numbers for the admin dashboard.
func (h *Handler) Stats(c *gin.Context) {
	utils.OK(c, collectStats())
}

func collectStats() gin.H {
	var users, posts, pages, products, orders, comments, messages int64
	database.DB.Model(&models.User{}).Count(&users)
	database.DB.Model(&models.Post{}).Count(&posts)
	database.DB.Model(&models.Page{}).Count(&pages)
	database.DB.Model(&models.Product{}).Count(&products)
	database.DB.Model(&models.Order{}).Count(&orders)
	database.DB.Model(&models.Comment{}).Where("status = ?", "pending").Count(&comments)
	database.DB.Model(&models.ContactMessage{}).Where("is_read = false").Count(&messages)

	var revenue float64
	database.DB.Model(&models.Order{}).
		Where("status IN ?", []string{"paid", "processing", "shipped", "completed"}).
		Select("COALESCE(SUM(total),0)").Scan(&revenue)

	since := time.Now().AddDate(0, 0, -30)
	var newUsers, newOrders int64
	database.DB.Model(&models.User{}).Where("created_at >= ?", since).Count(&newUsers)
	database.DB.Model(&models.Order{}).Where("created_at >= ?", since).Count(&newOrders)

	return gin.H{
		"users":            users,
		"new_users_30d":    newUsers,
		"posts":            posts,
		"pages":            pages,
		"products":         products,
		"orders":           orders,
		"new_orders_30d":   newOrders,
		"revenue":          revenue,
		"pending_comments": comments,
		"unread_messages":  messages,
		"online_clients":   realtime.H.Count(),
	}
}
