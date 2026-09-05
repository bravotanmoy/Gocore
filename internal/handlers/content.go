package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// uniqueSlug appends -2, -3... until the slug is free in the given table.
func uniqueSlug(table, base string, excludeID uint) string {
	slug := base
	for i := 2; ; i++ {
		var count int64
		q := database.DB.Table(table).Where("slug = ?", slug)
		if excludeID > 0 {
			q = q.Where("id <> ?", excludeID)
		}
		q.Count(&count)
		if count == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// ---------- Posts ----------

// ListPosts handles GET /api/v1/posts (admin: all statuses).
func (h *Handler) ListPosts(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Post{}).Preload("Author").Preload("Category").Preload("Tags")

	if s := c.Query("search"); s != "" {
		q = q.Where("title LIKE ?", "%"+s+"%")
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if cat := c.Query("category_id"); cat != "" {
		q = q.Where("category_id = ?", cat)
	}

	var total int64
	q.Count(&total)
	var posts []models.Post
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&posts)
	utils.Paginated(c, posts, total, page, perPage)
}

// GetPost handles GET /api/v1/posts/:id
func (h *Handler) GetPost(c *gin.Context) {
	var post models.Post
	if err := database.DB.Preload("Author").Preload("Category").Preload("Tags").First(&post, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "post not found")
		return
	}
	utils.OK(c, post)
}

type postInput struct {
	Title         string   `json:"title" form:"title" binding:"required"`
	Excerpt       string   `json:"excerpt" form:"excerpt"`
	Body          string   `json:"body" form:"body"`
	FeaturedImage string   `json:"featured_image" form:"featured_image"`
	Status        string   `json:"status" form:"status"`
	CategoryID    *uint    `json:"category_id" form:"category_id"`
	Tags          []string `json:"tags" form:"tags"`
	MetaTitle     string   `json:"meta_title" form:"meta_title"`
	MetaDesc      string   `json:"meta_description" form:"meta_description"`
}

func (h *Handler) applyTags(post *models.Post, names []string) {
	if names == nil {
		return
	}
	var tags []models.Tag
	for _, n := range names {
		if n == "" {
			continue
		}
		var t models.Tag
		slug := utils.Slugify(n)
		if slug == "" {
			continue
		}
		database.DB.Where(models.Tag{Slug: slug}).Attrs(models.Tag{Name: n}).FirstOrCreate(&t)
		tags = append(tags, t)
	}
	_ = database.DB.Model(post).Association("Tags").Replace(tags)
}

// CreatePost handles POST /api/v1/posts
func (h *Handler) CreatePost(c *gin.Context) {
	var in postInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user := middleware.CurrentUser(c)
	status := in.Status
	if status == "" {
		status = "draft"
	}
	if status == "published" && !user.HasPermission("content.publish") {
		status = "draft"
	}
	post := models.Post{
		Title: in.Title, Slug: uniqueSlug("posts", utils.Slugify(in.Title), 0),
		Excerpt: in.Excerpt, Body: in.Body, FeaturedImage: in.FeaturedImage,
		Status: status, AuthorID: user.ID, CategoryID: in.CategoryID,
		MetaTitle: in.MetaTitle, MetaDesc: in.MetaDesc,
	}
	if status == "published" {
		now := time.Now()
		post.PublishedAt = &now
	}
	if err := database.DB.Create(&post).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create post")
		return
	}
	h.applyTags(&post, in.Tags)

	c.Set("activity", "created post \""+post.Title+"\"")
	c.Set("activity_entity", "post")
	c.Set("activity_entity_id", strconv.Itoa(int(post.ID)))
	realtime.H.Broadcast("post.created", gin.H{"id": post.ID, "title": post.Title, "status": post.Status})
	utils.Created(c, post)
}

// UpdatePost handles PUT /api/v1/posts/:id
func (h *Handler) UpdatePost(c *gin.Context) {
	var post models.Post
	if err := database.DB.First(&post, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "post not found")
		return
	}
	var in postInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user := middleware.CurrentUser(c)
	updates := map[string]interface{}{
		"title": in.Title, "excerpt": in.Excerpt, "body": in.Body,
		"featured_image": in.FeaturedImage, "category_id": in.CategoryID,
		"meta_title": in.MetaTitle, "meta_desc": in.MetaDesc,
	}
	if in.Title != post.Title {
		updates["slug"] = uniqueSlug("posts", utils.Slugify(in.Title), post.ID)
	}
	if in.Status != "" && in.Status != post.Status {
		if in.Status == "published" && !user.HasPermission("content.publish") {
			utils.Fail(c, http.StatusForbidden, "you cannot publish content")
			return
		}
		updates["status"] = in.Status
		if in.Status == "published" && post.PublishedAt == nil {
			now := time.Now()
			updates["published_at"] = &now
		}
	}
	database.DB.Model(&post).Updates(updates)
	h.applyTags(&post, in.Tags)

	c.Set("activity", "updated post \""+post.Title+"\"")
	c.Set("activity_entity", "post")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, post)
}

// DeletePost handles DELETE /api/v1/posts/:id
func (h *Handler) DeletePost(c *gin.Context) {
	var post models.Post
	if err := database.DB.First(&post, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "post not found")
		return
	}
	database.DB.Delete(&post)
	c.Set("activity", "deleted post \""+post.Title+"\"")
	c.Set("activity_entity", "post")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "post deleted"})
}

// ---------- Pages ----------

// ListPages handles GET /api/v1/pages
func (h *Handler) ListPages(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Page{}).Preload("Author")
	if s := c.Query("search"); s != "" {
		q = q.Where("title LIKE ?", "%"+s+"%")
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	var total int64
	q.Count(&total)
	var pages []models.Page
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&pages)
	utils.Paginated(c, pages, total, page, perPage)
}

type pageInput struct {
	Title     string `json:"title" form:"title" binding:"required"`
	Body      string `json:"body" form:"body"`
	Template  string `json:"template" form:"template"`
	Status    string `json:"status" form:"status"`
	MetaTitle string `json:"meta_title" form:"meta_title"`
	MetaDesc  string `json:"meta_description" form:"meta_description"`
}

// CreatePage handles POST /api/v1/pages
func (h *Handler) CreatePage(c *gin.Context) {
	var in pageInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user := middleware.CurrentUser(c)
	status := in.Status
	if status == "" {
		status = "draft"
	}
	tmpl := in.Template
	if tmpl == "" {
		tmpl = "default"
	}
	page := models.Page{
		Title: in.Title, Slug: uniqueSlug("pages", utils.Slugify(in.Title), 0),
		Body: in.Body, Template: tmpl, Status: status, AuthorID: user.ID,
		MetaTitle: in.MetaTitle, MetaDesc: in.MetaDesc,
	}
	if err := database.DB.Create(&page).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create page")
		return
	}
	c.Set("activity", "created page \""+page.Title+"\"")
	c.Set("activity_entity", "page")
	c.Set("activity_entity_id", strconv.Itoa(int(page.ID)))
	utils.Created(c, page)
}

// UpdatePage handles PUT /api/v1/pages/:id
func (h *Handler) UpdatePage(c *gin.Context) {
	var page models.Page
	if err := database.DB.First(&page, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "page not found")
		return
	}
	var in pageInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{
		"title": in.Title, "body": in.Body,
		"meta_title": in.MetaTitle, "meta_desc": in.MetaDesc,
	}
	if in.Template != "" {
		updates["template"] = in.Template
	}
	if in.Status != "" {
		updates["status"] = in.Status
	}
	if in.Title != page.Title {
		updates["slug"] = uniqueSlug("pages", utils.Slugify(in.Title), page.ID)
	}
	database.DB.Model(&page).Updates(updates)
	c.Set("activity", "updated page \""+page.Title+"\"")
	c.Set("activity_entity", "page")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, page)
}

// DeletePage handles DELETE /api/v1/pages/:id
func (h *Handler) DeletePage(c *gin.Context) {
	var page models.Page
	if err := database.DB.First(&page, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "page not found")
		return
	}
	database.DB.Delete(&page)
	c.Set("activity", "deleted page \""+page.Title+"\"")
	c.Set("activity_entity", "page")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "page deleted"})
}

// ---------- Categories ----------

// ListCategories handles GET /api/v1/categories?type=post|product
func (h *Handler) ListCategories(c *gin.Context) {
	q := database.DB.Model(&models.Category{})
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	var cats []models.Category
	q.Order("id").Find(&cats)
	utils.OK(c, cats)
}

type categoryInput struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Description string `json:"description" form:"description"`
	Type        string `json:"type" form:"type"`
	ParentID    *uint  `json:"parent_id" form:"parent_id"`
}

// CreateCategory handles POST /api/v1/categories
func (h *Handler) CreateCategory(c *gin.Context) {
	var in categoryInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	t := in.Type
	if t == "" {
		t = "post"
	}
	cat := models.Category{
		Name: in.Name, Slug: uniqueSlug("categories", utils.Slugify(in.Name), 0),
		Description: in.Description, Type: t, ParentID: in.ParentID,
	}
	if err := database.DB.Create(&cat).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create category")
		return
	}
	c.Set("activity", "created category "+cat.Name)
	c.Set("activity_entity", "category")
	c.Set("activity_entity_id", strconv.Itoa(int(cat.ID)))
	utils.Created(c, cat)
}

// UpdateCategory handles PUT /api/v1/categories/:id
func (h *Handler) UpdateCategory(c *gin.Context) {
	var cat models.Category
	if err := database.DB.First(&cat, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "category not found")
		return
	}
	var in categoryInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{"name": in.Name, "description": in.Description, "parent_id": in.ParentID}
	if in.Name != cat.Name {
		updates["slug"] = uniqueSlug("categories", utils.Slugify(in.Name), cat.ID)
	}
	database.DB.Model(&cat).Updates(updates)
	c.Set("activity", "updated category "+cat.Name)
	c.Set("activity_entity", "category")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, cat)
}

// DeleteCategory handles DELETE /api/v1/categories/:id
func (h *Handler) DeleteCategory(c *gin.Context) {
	var cat models.Category
	if err := database.DB.First(&cat, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "category not found")
		return
	}
	database.DB.Model(&models.Post{}).Where("category_id = ?", cat.ID).Update("category_id", nil)
	database.DB.Model(&models.Product{}).Where("category_id = ?", cat.ID).Update("category_id", nil)
	database.DB.Model(&models.Category{}).Where("parent_id = ?", cat.ID).Update("parent_id", nil)
	database.DB.Delete(&cat)
	c.Set("activity", "deleted category "+cat.Name)
	c.Set("activity_entity", "category")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "category deleted"})
}

// ---------- Comments ----------

// ListComments handles GET /api/v1/comments?status=&post_id=
func (h *Handler) ListComments(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Comment{}).Preload("User")
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if pid := c.Query("post_id"); pid != "" {
		q = q.Where("post_id = ?", pid)
	}
	var total int64
	q.Count(&total)
	var comments []models.Comment
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&comments)
	utils.Paginated(c, comments, total, page, perPage)
}

// ModerateComment handles PUT /api/v1/comments/:id {status: approved|pending|spam}
func (h *Handler) ModerateComment(c *gin.Context) {
	var comment models.Comment
	if err := database.DB.First(&comment, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "comment not found")
		return
	}
	var in struct {
		Status string `json:"status" binding:"required,oneof=approved pending spam"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	database.DB.Model(&comment).Update("status", in.Status)
	c.Set("activity", "moderated comment #"+c.Param("id")+" -> "+in.Status)
	c.Set("activity_entity", "comment")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, comment)
}

// DeleteComment handles DELETE /api/v1/comments/:id
func (h *Handler) DeleteComment(c *gin.Context) {
	database.DB.Delete(&models.Comment{}, c.Param("id"))
	c.Set("activity", "deleted comment #"+c.Param("id"))
	c.Set("activity_entity", "comment")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "comment deleted"})
}
