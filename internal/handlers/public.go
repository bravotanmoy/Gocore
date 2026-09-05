package handlers

import (
	"net/http"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Public endpoints power headless frontends (Next/Nuxt/plain HTML) without auth.

// PublicPosts handles GET /api/v1/public/posts — published posts only.
func (h *Handler) PublicPosts(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Post{}).Preload("Author").Preload("Category").Preload("Tags").
		Where("status = ?", "published")
	if s := c.Query("search"); s != "" {
		q = q.Where("title LIKE ?", "%"+s+"%")
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Joins("JOIN categories ON categories.id = posts.category_id").
			Where("categories.slug = ?", cat)
	}
	if tag := c.Query("tag"); tag != "" {
		q = q.Joins("JOIN post_tags pt ON pt.post_id = posts.id").
			Joins("JOIN tags ON tags.id = pt.tag_id").
			Where("tags.slug = ?", tag)
	}
	var total int64
	q.Count(&total)
	var posts []models.Post
	q.Order("published_at DESC").Limit(perPage).Offset(offset).Find(&posts)
	utils.Paginated(c, posts, total, page, perPage)
}

// PublicPost handles GET /api/v1/public/posts/:slug — also counts the view.
func (h *Handler) PublicPost(c *gin.Context) {
	var post models.Post
	if err := database.DB.Preload("Author").Preload("Category").Preload("Tags").
		Where("slug = ? AND status = ?", c.Param("slug"), "published").First(&post).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "post not found")
		return
	}
	database.DB.Model(&post).UpdateColumn("views", gorm.Expr("views + 1"))

	var comments []models.Comment
	database.DB.Preload("User").Where("post_id = ? AND status = ?", post.ID, "approved").
		Order("id").Find(&comments)
	utils.OK(c, gin.H{"post": post, "comments": comments})
}

// PublicPage handles GET /api/v1/public/pages/:slug
func (h *Handler) PublicPage(c *gin.Context) {
	var page models.Page
	if err := database.DB.Where("slug = ? AND status = ?", c.Param("slug"), "published").First(&page).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "page not found")
		return
	}
	utils.OK(c, page)
}

// PublicProducts handles GET /api/v1/public/products — active products only.
func (h *Handler) PublicProducts(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Product{}).Preload("Category").Where("status = ?", "active")
	if s := c.Query("search"); s != "" {
		q = q.Where("name LIKE ?", "%"+s+"%")
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Joins("JOIN categories ON categories.id = products.category_id").
			Where("categories.slug = ?", cat)
	}
	var total int64
	q.Count(&total)
	var products []models.Product
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&products)
	utils.Paginated(c, products, total, page, perPage)
}

// PublicProduct handles GET /api/v1/public/products/:slug
func (h *Handler) PublicProduct(c *gin.Context) {
	var p models.Product
	if err := database.DB.Preload("Category").
		Where("slug = ? AND status = ?", c.Param("slug"), "active").First(&p).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "product not found")
		return
	}
	utils.OK(c, p)
}

// PublicCategories handles GET /api/v1/public/categories?type=post|product
func (h *Handler) PublicCategories(c *gin.Context) {
	q := database.DB.Model(&models.Category{})
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	var cats []models.Category
	q.Order("name").Find(&cats)
	utils.OK(c, cats)
}

// PublicMenu handles GET /api/v1/public/menus/:name — items sorted for rendering.
func (h *Handler) PublicMenu(c *gin.Context) {
	var menu models.Menu
	if err := database.DB.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order")
	}).Where("name = ?", c.Param("name")).First(&menu).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "menu not found")
		return
	}
	utils.OK(c, menu)
}

// publicSettingKeys whitelists what unauthenticated clients may read.
var publicSettingKeys = map[string]bool{
	"site_name": true, "site_tagline": true, "site_url": true,
	"posts_per_page": true, "currency": true, "allow_registration": true,
}

// PublicSettings handles GET /api/v1/public/settings
func (h *Handler) PublicSettings(c *gin.Context) {
	var settings []models.Setting
	database.DB.Find(&settings)
	out := map[string]string{}
	for _, s := range settings {
		if publicSettingKeys[s.Key] {
			out[s.Key] = s.Value
		}
	}
	utils.OK(c, out)
}

// PublicContact handles POST /api/v1/public/contact — contact form submissions.
func (h *Handler) PublicContact(c *gin.Context) {
	var in struct {
		Name    string `json:"name" form:"name" binding:"required"`
		Email   string `json:"email" form:"email" binding:"required,email"`
		Phone   string `json:"phone" form:"phone"`
		Subject string `json:"subject" form:"subject"`
		Message string `json:"message" form:"message" binding:"required"`
	}
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	msg := models.ContactMessage{
		Name: in.Name, Email: in.Email, Phone: in.Phone,
		Subject: in.Subject, Message: in.Message,
	}
	database.DB.Create(&msg)
	realtime.H.Broadcast("contact.received", gin.H{"id": msg.ID, "name": msg.Name, "subject": msg.Subject})
	utils.Created(c, gin.H{"message": "thanks, we received your message"})
}

// PublicComment handles POST /api/v1/public/posts/:slug/comments — goes to moderation.
func (h *Handler) PublicComment(c *gin.Context) {
	var post models.Post
	if err := database.DB.Where("slug = ? AND status = ?", c.Param("slug"), "published").First(&post).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "post not found")
		return
	}
	var in struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Body     string `json:"body" binding:"required"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	comment := models.Comment{
		PostID: post.ID, Name: in.Name, Email: in.Email,
		Body: in.Body, ParentID: in.ParentID, Status: "pending",
	}
	database.DB.Create(&comment)
	utils.Created(c, gin.H{"message": "comment submitted for moderation"})
}

// PublicOrder handles POST /api/v1/public/orders — headless storefront checkout.
func (h *Handler) PublicOrder(c *gin.Context) {
	var in orderInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	order, msg := h.buildOrder(in, nil)
	if msg != "" {
		utils.Fail(c, http.StatusBadRequest, msg)
		return
	}
	utils.Created(c, order)
}
