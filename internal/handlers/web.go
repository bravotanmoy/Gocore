package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newUUID() string { return uuid.NewString() }

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

// pageData builds the common template payload for admin pages.
func pageData(c *gin.Context, title string, extra gin.H) gin.H {
	data := gin.H{
		"title": title,
		"user":  middleware.CurrentUser(c),
		"flash": c.Query("flash"),
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func webPage(c *gin.Context) int {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if p < 1 {
		p = 1
	}
	return p
}

// ---------- Dashboard ----------

// CMSDashboard renders the CMS overview with live stats.
func (h *Handler) CMSDashboard(c *gin.Context) {
	var recent []models.ActivityLog
	database.DB.Preload("User").Order("id DESC").Limit(8).Find(&recent)
	var latestUsers []models.User
	database.DB.Order("id DESC").Limit(5).Find(&latestUsers)
	c.HTML(http.StatusOK, "cms-dashboard.html", pageData(c, "CMS Overview", gin.H{
		"stats":       collectStats(),
		"recent":      recent,
		"latestUsers": latestUsers,
	}))
}

// ---------- Users ----------

const webPerPage = 15

// CMSUsers renders the user list.
func (h *Handler) CMSUsers(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.User{}).Preload("Roles")
	search := c.Query("search")
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var users []models.User
	q.Order("id DESC").Limit(webPerPage).Offset((page - 1) * webPerPage).Find(&users)
	c.HTML(http.StatusOK, "cms-users.html", pageData(c, "Users", gin.H{
		"users": users, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*webPerPage) < total,
		"prevPage": page - 1, "nextPage": page + 1, "search": search,
	}))
}

// CMSUserForm renders the create/edit user form.
func (h *Handler) CMSUserForm(c *gin.Context) {
	var target models.User
	title := "New User"
	if id := c.Param("id"); id != "" {
		if err := database.DB.Preload("Roles").First(&target, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=User not found")
			return
		}
		title = "Edit User"
	}
	var roles []models.Role
	database.DB.Order("id").Find(&roles)
	has := map[uint]bool{}
	for _, r := range target.Roles {
		has[r.ID] = true
	}
	type roleView struct {
		models.Role
		Checked bool
	}
	var views []roleView
	for _, r := range roles {
		views = append(views, roleView{r, has[r.ID]})
	}
	c.HTML(http.StatusOK, "cms-user-form.html", pageData(c, title, gin.H{
		"target": target, "roles": views, "isEdit": target.ID != 0,
	}))
}

// CMSUserSave handles the user form POST (create or update).
func (h *Handler) CMSUserSave(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	status := c.DefaultPostForm("status", "active")
	var roleIDs []uint
	for _, v := range c.PostFormArray("role_ids") {
		if n, err := strconv.Atoi(v); err == nil {
			roleIDs = append(roleIDs, uint(n))
		}
	}
	if roleIDs == nil {
		roleIDs = []uint{}
	}

	id := c.Param("id")
	if name == "" || email == "" {
		c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=Name and email are required")
		return
	}

	if id == "" { // create
		if password == "" {
			c.Redirect(http.StatusFound, "/dashboard/cms/users/new?flash=Password is required")
			return
		}
		var exists int64
		database.DB.Model(&models.User{}).Where("email = ?", email).Count(&exists)
		if exists > 0 {
			c.Redirect(http.StatusFound, "/dashboard/cms/users/new?flash=Email already registered")
			return
		}
		in := userInput{Name: name, Email: email, Password: password, Phone: c.PostForm("phone"), Status: status, RoleIDs: roleIDs}
		hash, _ := utils.HashPassword(in.Password)
		user := models.User{Name: in.Name, Email: in.Email, Password: hash, Phone: in.Phone, Status: in.Status}
		user.UUID = newUUID()
		if err := database.DB.Create(&user).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=Could not create user")
			return
		}
		h.syncRoles(&user, roleIDs)
		c.Set("activity", "created user "+user.Email)
		c.Set("activity_entity", "user")
	} else { // update
		var user models.User
		if err := database.DB.First(&user, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=User not found")
			return
		}
		updates := map[string]interface{}{"name": name, "email": email, "phone": c.PostForm("phone"), "status": status}
		if password != "" {
			hash, _ := utils.HashPassword(password)
			updates["password"] = hash
		}
		database.DB.Model(&user).Updates(updates)
		h.syncRoles(&user, roleIDs)
		c.Set("activity", "updated user "+user.Email)
		c.Set("activity_entity", "user")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=Saved")
}

// CMSUserDelete handles POST /dashboard/cms/users/:id/delete.
func (h *Handler) CMSUserDelete(c *gin.Context) {
	me := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	if me.ID == uint(id) {
		c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=You cannot delete yourself")
		return
	}
	var user models.User
	if database.DB.First(&user, id).Error == nil {
		database.DB.Delete(&user)
		c.Set("activity", "deleted user "+user.Email)
		c.Set("activity_entity", "user")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/users?flash=User deleted")
}

// ---------- Roles ----------

// CMSRoles renders the role list.
func (h *Handler) CMSRoles(c *gin.Context) {
	var roles []models.Role
	database.DB.Preload("Permissions").Order("id").Find(&roles)
	counts := map[uint]int64{}
	for _, r := range roles {
		var n int64
		database.DB.Table("user_roles").Where("role_id = ?", r.ID).Count(&n)
		counts[r.ID] = n
	}
	c.HTML(http.StatusOK, "cms-roles.html", pageData(c, "Roles & Permissions", gin.H{
		"roles": roles, "counts": counts,
	}))
}

type permView struct {
	ID      uint
	Name    string
	Checked bool
}

type permGroup struct {
	Group string
	Perms []permView
}

// CMSRoleForm renders the create/edit role form with a permission matrix.
func (h *Handler) CMSRoleForm(c *gin.Context) {
	var role models.Role
	title := "New Role"
	if id := c.Param("id"); id != "" {
		if err := database.DB.Preload("Permissions").First(&role, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Role not found")
			return
		}
		title = "Edit Role: " + role.DisplayName
	}
	has := map[uint]bool{}
	for _, p := range role.Permissions {
		has[p.ID] = true
	}
	var perms []models.Permission
	database.DB.Order("perm_group, name").Find(&perms)
	var groups []permGroup
	idx := map[string]int{}
	for _, p := range perms {
		if _, ok := idx[p.Group]; !ok {
			idx[p.Group] = len(groups)
			groups = append(groups, permGroup{Group: p.Group})
		}
		g := idx[p.Group]
		groups[g].Perms = append(groups[g].Perms, permView{p.ID, p.Name, has[p.ID]})
	}
	c.HTML(http.StatusOK, "cms-role-form.html", pageData(c, title, gin.H{
		"role": role, "groups": groups, "isEdit": role.ID != 0,
	}))
}

// CMSRoleSave handles the role form POST.
func (h *Handler) CMSRoleSave(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	display := strings.TrimSpace(c.PostForm("display_name"))
	desc := c.PostForm("description")
	var permIDs []uint
	for _, v := range c.PostFormArray("permission_ids") {
		if n, err := strconv.Atoi(v); err == nil {
			permIDs = append(permIDs, uint(n))
		}
	}
	if permIDs == nil {
		permIDs = []uint{}
	}

	id := c.Param("id")
	if id == "" {
		if name == "" {
			c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Role name is required")
			return
		}
		slug := utils.Slugify(name)
		var exists int64
		database.DB.Model(&models.Role{}).Where("name = ?", slug).Count(&exists)
		if exists > 0 {
			c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Role already exists")
			return
		}
		if display == "" {
			display = name
		}
		role := models.Role{Name: slug, DisplayName: display, Description: desc}
		database.DB.Create(&role)
		h.syncPermissions(&role, permIDs)
		c.Set("activity", "created role "+role.Name)
		c.Set("activity_entity", "role")
	} else {
		var role models.Role
		if err := database.DB.First(&role, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Role not found")
			return
		}
		updates := map[string]interface{}{"description": desc}
		if display != "" {
			updates["display_name"] = display
		}
		if !role.IsSystem && name != "" {
			updates["name"] = utils.Slugify(name)
		}
		database.DB.Model(&role).Updates(updates)
		h.syncPermissions(&role, permIDs)
		c.Set("activity", "updated role "+role.Name)
		c.Set("activity_entity", "role")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Saved")
}

// CMSRoleDelete handles POST /dashboard/cms/roles/:id/delete.
func (h *Handler) CMSRoleDelete(c *gin.Context) {
	var role models.Role
	if err := database.DB.First(&role, c.Param("id")).Error; err == nil {
		if role.IsSystem {
			c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=System roles cannot be deleted")
			return
		}
		_ = database.DB.Model(&role).Association("Permissions").Clear()
		database.DB.Exec("DELETE FROM user_roles WHERE role_id = ?", role.ID)
		database.DB.Delete(&role)
		c.Set("activity", "deleted role "+role.Name)
		c.Set("activity_entity", "role")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/roles?flash=Role deleted")
}

// ---------- Activity log ----------

// CMSActivity renders the activity log with filters.
func (h *Handler) CMSActivity(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.ActivityLog{}).Preload("User")
	action := c.Query("action")
	entity := c.Query("entity")
	search := c.Query("search")
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if entity != "" {
		q = q.Where("entity = ?", entity)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("detail LIKE ? OR path LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var logs []models.ActivityLog
	q.Order("id DESC").Limit(30).Offset((page - 1) * 30).Find(&logs)
	c.HTML(http.StatusOK, "cms-activity.html", pageData(c, "Activity Log", gin.H{
		"logs": logs, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*30) < total,
		"prevPage": page - 1, "nextPage": page + 1,
		"action": action, "entity": entity, "search": search,
	}))
}

// ---------- Posts ----------

// CMSPosts renders the post list.
func (h *Handler) CMSPosts(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.Post{}).Preload("Author").Preload("Category")
	search := c.Query("search")
	status := c.Query("status")
	if search != "" {
		q = q.Where("title LIKE ?", "%"+search+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var posts []models.Post
	q.Order("id DESC").Limit(webPerPage).Offset((page - 1) * webPerPage).Find(&posts)
	c.HTML(http.StatusOK, "cms-posts.html", pageData(c, "Posts", gin.H{
		"posts": posts, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*webPerPage) < total,
		"prevPage": page - 1, "nextPage": page + 1,
		"search": search, "status": status,
	}))
}

// CMSPostForm renders the create/edit post form.
func (h *Handler) CMSPostForm(c *gin.Context) {
	var post models.Post
	title := "New Post"
	if id := c.Param("id"); id != "" {
		if err := database.DB.Preload("Tags").First(&post, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/posts?flash=Post not found")
			return
		}
		title = "Edit Post"
	}
	var cats []models.Category
	database.DB.Where("type = ?", "post").Order("name").Find(&cats)
	var tagNames []string
	for _, t := range post.Tags {
		tagNames = append(tagNames, t.Name)
	}
	c.HTML(http.StatusOK, "cms-post-form.html", pageData(c, title, gin.H{
		"post": post, "categories": cats, "isEdit": post.ID != 0,
		"tags":       strings.Join(tagNames, ", "),
		"canPublish": middleware.CurrentUser(c).HasPermission("content.publish"),
	}))
}

// CMSPostSave handles the post form POST.
func (h *Handler) CMSPostSave(c *gin.Context) {
	user := middleware.CurrentUser(c)
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		c.Redirect(http.StatusFound, "/dashboard/cms/posts?flash=Title is required")
		return
	}
	status := c.DefaultPostForm("status", "draft")
	if status == "published" && !user.HasPermission("content.publish") {
		status = "draft"
	}
	var catID *uint
	if v, err := strconv.Atoi(c.PostForm("category_id")); err == nil && v > 0 {
		u := uint(v)
		catID = &u
	}
	var tags []string
	for _, t := range strings.Split(c.PostForm("tags"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}

	id := c.Param("id")
	if id == "" {
		post := models.Post{
			Title: title, Slug: uniqueSlug("posts", utils.Slugify(title), 0),
			Excerpt: c.PostForm("excerpt"), Body: c.PostForm("body"),
			FeaturedImage: c.PostForm("featured_image"), Status: status,
			AuthorID: user.ID, CategoryID: catID,
			MetaTitle: c.PostForm("meta_title"), MetaDesc: c.PostForm("meta_description"),
		}
		if status == "published" {
			now := nowPtr()
			post.PublishedAt = now
		}
		database.DB.Create(&post)
		h.applyTags(&post, tags)
		c.Set("activity", "created post \""+post.Title+"\"")
		c.Set("activity_entity", "post")
	} else {
		var post models.Post
		if err := database.DB.First(&post, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/posts?flash=Post not found")
			return
		}
		updates := map[string]interface{}{
			"title": title, "excerpt": c.PostForm("excerpt"), "body": c.PostForm("body"),
			"featured_image": c.PostForm("featured_image"), "category_id": catID,
			"meta_title": c.PostForm("meta_title"), "meta_desc": c.PostForm("meta_description"),
			"status": status,
		}
		if title != post.Title {
			updates["slug"] = uniqueSlug("posts", utils.Slugify(title), post.ID)
		}
		if status == "published" && post.PublishedAt == nil {
			updates["published_at"] = nowPtr()
		}
		database.DB.Model(&post).Updates(updates)
		h.applyTags(&post, tags)
		c.Set("activity", "updated post \""+post.Title+"\"")
		c.Set("activity_entity", "post")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/posts?flash=Saved")
}

// CMSPostDelete handles POST /dashboard/cms/posts/:id/delete.
func (h *Handler) CMSPostDelete(c *gin.Context) {
	var post models.Post
	if database.DB.First(&post, c.Param("id")).Error == nil {
		database.DB.Delete(&post)
		c.Set("activity", "deleted post \""+post.Title+"\"")
		c.Set("activity_entity", "post")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/posts?flash=Post deleted")
}

// ---------- Categories ----------

// CMSCategories renders category management (list + quick add).
func (h *Handler) CMSCategories(c *gin.Context) {
	var cats []models.Category
	database.DB.Order("type, name").Find(&cats)
	c.HTML(http.StatusOK, "cms-categories.html", pageData(c, "Categories", gin.H{"categories": cats}))
}

// CMSCategorySave handles the quick-add category POST.
func (h *Handler) CMSCategorySave(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "/dashboard/cms/categories?flash=Name is required")
		return
	}
	t := c.DefaultPostForm("type", "post")
	cat := models.Category{
		Name: name, Slug: uniqueSlug("categories", utils.Slugify(name), 0),
		Description: c.PostForm("description"), Type: t,
	}
	database.DB.Create(&cat)
	c.Set("activity", "created category "+cat.Name)
	c.Set("activity_entity", "category")
	c.Redirect(http.StatusFound, "/dashboard/cms/categories?flash=Saved")
}

// CMSCategoryDelete handles POST /dashboard/cms/categories/:id/delete.
func (h *Handler) CMSCategoryDelete(c *gin.Context) {
	var cat models.Category
	if database.DB.First(&cat, c.Param("id")).Error == nil {
		database.DB.Model(&models.Post{}).Where("category_id = ?", cat.ID).Update("category_id", nil)
		database.DB.Model(&models.Product{}).Where("category_id = ?", cat.ID).Update("category_id", nil)
		database.DB.Model(&models.Category{}).Where("parent_id = ?", cat.ID).Update("parent_id", nil)
		database.DB.Delete(&cat)
		c.Set("activity", "deleted category "+cat.Name)
		c.Set("activity_entity", "category")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/categories?flash=Category deleted")
}

// ---------- Media ----------

// CMSMedia renders the media library.
func (h *Handler) CMSMedia(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.Media{})
	var total int64
	q.Count(&total)
	var items []models.Media
	q.Order("id DESC").Limit(24).Offset((page - 1) * 24).Find(&items)
	c.HTML(http.StatusOK, "cms-media.html", pageData(c, "Media Library", gin.H{
		"items": items, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*24) < total,
		"prevPage": page - 1, "nextPage": page + 1,
	}))
}

// CMSMediaUpload handles the media upload form POST, then redirects back.
func (h *Handler) CMSMediaUpload(c *gin.Context) {
	_, errMsg := h.saveUpload(c)
	if errMsg != "" {
		c.Redirect(http.StatusFound, "/dashboard/cms/media?flash="+errMsg)
		return
	}
	c.Set("activity", "uploaded a file")
	c.Set("activity_entity", "media")
	c.Redirect(http.StatusFound, "/dashboard/cms/media?flash=Uploaded")
}

// CMSMediaDelete handles POST /dashboard/cms/media/:id/delete.
func (h *Handler) CMSMediaDelete(c *gin.Context) {
	var media models.Media
	if database.DB.First(&media, c.Param("id")).Error == nil {
		removeUploadFile(h.Cfg.UploadDir, media.Path)
		database.DB.Delete(&media)
		c.Set("activity", "deleted file "+media.FileName)
		c.Set("activity_entity", "media")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/media?flash=Deleted")
}

// ---------- Products ----------

// CMSProducts renders the product list.
func (h *Handler) CMSProducts(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.Product{}).Preload("Category")
	search := c.Query("search")
	if search != "" {
		q = q.Where("name LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	var total int64
	q.Count(&total)
	var products []models.Product
	q.Order("id DESC").Limit(webPerPage).Offset((page - 1) * webPerPage).Find(&products)
	c.HTML(http.StatusOK, "cms-products.html", pageData(c, "Products", gin.H{
		"products": products, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*webPerPage) < total,
		"prevPage": page - 1, "nextPage": page + 1, "search": search,
	}))
}

// CMSProductForm renders the create/edit product form.
func (h *Handler) CMSProductForm(c *gin.Context) {
	var p models.Product
	title := "New Product"
	if id := c.Param("id"); id != "" {
		if err := database.DB.First(&p, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/products?flash=Product not found")
			return
		}
		title = "Edit Product"
	}
	var cats []models.Category
	database.DB.Where("type = ?", "product").Order("name").Find(&cats)
	c.HTML(http.StatusOK, "cms-product-form.html", pageData(c, title, gin.H{
		"product": p, "categories": cats, "isEdit": p.ID != 0,
	}))
}

// CMSProductSave handles the product form POST.
func (h *Handler) CMSProductSave(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "/dashboard/cms/products?flash=Name is required")
		return
	}
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	var salePrice *float64
	if v, err := strconv.ParseFloat(c.PostForm("sale_price"), 64); err == nil && v > 0 {
		salePrice = &v
	}
	var catID *uint
	if v, err := strconv.Atoi(c.PostForm("category_id")); err == nil && v > 0 {
		u := uint(v)
		catID = &u
	}
	status := c.DefaultPostForm("status", "active")

	id := c.Param("id")
	if id == "" {
		p := models.Product{
			Name: name, Slug: uniqueSlug("products", utils.Slugify(name), 0),
			SKU: c.PostForm("sku"), Description: c.PostForm("description"),
			Price: price, SalePrice: salePrice, Stock: stock,
			Image: c.PostForm("image"), Status: status, CategoryID: catID,
		}
		database.DB.Create(&p)
		c.Set("activity", "created product "+p.Name)
		c.Set("activity_entity", "product")
	} else {
		var p models.Product
		if err := database.DB.First(&p, id).Error; err != nil {
			c.Redirect(http.StatusFound, "/dashboard/cms/products?flash=Product not found")
			return
		}
		updates := map[string]interface{}{
			"name": name, "sku": c.PostForm("sku"), "description": c.PostForm("description"),
			"price": price, "sale_price": salePrice, "stock": stock,
			"image": c.PostForm("image"), "status": status, "category_id": catID,
		}
		if name != p.Name {
			updates["slug"] = uniqueSlug("products", utils.Slugify(name), p.ID)
		}
		database.DB.Model(&p).Updates(updates)
		c.Set("activity", "updated product "+p.Name)
		c.Set("activity_entity", "product")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/products?flash=Saved")
}

// CMSProductDelete handles POST /dashboard/cms/products/:id/delete.
func (h *Handler) CMSProductDelete(c *gin.Context) {
	var p models.Product
	if database.DB.First(&p, c.Param("id")).Error == nil {
		database.DB.Delete(&p)
		c.Set("activity", "deleted product "+p.Name)
		c.Set("activity_entity", "product")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/products?flash=Product deleted")
}

// ---------- Orders ----------

// CMSOrders renders the order list.
func (h *Handler) CMSOrders(c *gin.Context) {
	page := webPage(c)
	q := database.DB.Model(&models.Order{}).Preload("Items")
	status := c.Query("status")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var orders []models.Order
	q.Order("id DESC").Limit(webPerPage).Offset((page - 1) * webPerPage).Find(&orders)
	c.HTML(http.StatusOK, "cms-orders.html", pageData(c, "Orders", gin.H{
		"orders": orders, "total": total, "page": page,
		"hasPrev": page > 1, "hasNext": int64(page*webPerPage) < total,
		"prevPage": page - 1, "nextPage": page + 1, "status": status,
		"statuses": []string{"pending", "paid", "processing", "shipped", "completed", "cancelled", "refunded"},
	}))
}

// CMSOrderStatus handles POST /dashboard/cms/orders/:id/status.
func (h *Handler) CMSOrderStatus(c *gin.Context) {
	status := c.PostForm("status")
	if !validOrderStatus[status] {
		c.Redirect(http.StatusFound, "/dashboard/cms/orders?flash=Invalid status")
		return
	}
	var o models.Order
	if database.DB.First(&o, c.Param("id")).Error == nil {
		database.DB.Model(&o).Update("status", status)
		c.Set("activity", "order "+o.OrderNumber+" -> "+status)
		c.Set("activity_entity", "order")
	}
	c.Redirect(http.StatusFound, "/dashboard/cms/orders?flash=Order updated")
}

// ---------- Settings ----------

// CMSSettings renders grouped site settings.
func (h *Handler) CMSSettings(c *gin.Context) {
	var settings []models.Setting
	database.DB.Order("setting_key").Find(&settings)
	grouped := map[string][]models.Setting{}
	for _, s := range settings {
		grouped[s.Group] = append(grouped[s.Group], s)
	}
	c.HTML(http.StatusOK, "cms-settings.html", pageData(c, "Site Settings", gin.H{"groups": grouped}))
}

// CMSSettingsSave handles the settings form POST (fields named set_<key>).
func (h *Handler) CMSSettingsSave(c *gin.Context) {
	_ = c.Request.ParseForm()
	for key, values := range c.Request.PostForm {
		if !strings.HasPrefix(key, "set_") || len(values) == 0 {
			continue
		}
		k := strings.TrimPrefix(key, "set_")
		database.DB.Model(&models.Setting{}).Where("setting_key = ?", k).Update("value", values[0])
	}
	c.Set("activity", "updated site settings")
	c.Set("activity_entity", "setting")
	c.Redirect(http.StatusFound, "/dashboard/cms/settings?flash=Settings saved")
}

// ---------- Profile ----------

// CMSProfile renders the current user's profile page.
func (h *Handler) CMSProfile(c *gin.Context) {
	c.HTML(http.StatusOK, "cms-profile.html", pageData(c, "My Profile", gin.H{}))
}

// CMSProfileSave updates name/phone or password from the profile form.
func (h *Handler) CMSProfileSave(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if name := strings.TrimSpace(c.PostForm("name")); name != "" {
		database.DB.Model(user).Updates(map[string]interface{}{"name": name, "phone": c.PostForm("phone")})
	}
	current := c.PostForm("current_password")
	newPass := c.PostForm("new_password")
	if newPass != "" {
		if !utils.CheckPassword(user.Password, current) {
			c.Redirect(http.StatusFound, "/dashboard/cms/profile?flash=Current password is incorrect")
			return
		}
		if len(newPass) < 6 {
			c.Redirect(http.StatusFound, "/dashboard/cms/profile?flash=New password must be at least 6 characters")
			return
		}
		hash, _ := utils.HashPassword(newPass)
		database.DB.Model(user).Update("password", hash)
		database.DB.Model(&models.RefreshToken{}).Where("user_id = ?", user.ID).Update("revoked", true)
	}
	c.Set("activity", "updated own profile")
	c.Set("activity_entity", "user")
	c.Redirect(http.StatusFound, "/dashboard/cms/profile?flash=Profile saved")
}
