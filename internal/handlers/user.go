package handlers

import (
	"net/http"
	"strconv"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListUsers handles GET /api/v1/users?search=&role=&status=&page=
func (h *Handler) ListUsers(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.User{}).Preload("Roles")

	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if role := c.Query("role"); role != "" {
		q = q.Joins("JOIN user_roles ur ON ur.user_id = users.id").
			Joins("JOIN roles r ON r.id = ur.role_id").
			Where("r.name = ?", role)
	}

	var total int64
	q.Count(&total)
	var users []models.User
	q.Order("users.id DESC").Limit(perPage).Offset(offset).Find(&users)
	utils.Paginated(c, users, total, page, perPage)
}

// GetUser handles GET /api/v1/users/:id
func (h *Handler) GetUser(c *gin.Context) {
	var user models.User
	if err := database.DB.Preload("Roles.Permissions").First(&user, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "user not found")
		return
	}
	utils.OK(c, user)
}

type userInput struct {
	Name     string `json:"name" form:"name" binding:"required"`
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password"`
	Phone    string `json:"phone" form:"phone"`
	Status   string `json:"status" form:"status"`
	RoleIDs  []uint `json:"role_ids" form:"role_ids"`
}

// CreateUser handles POST /api/v1/users
func (h *Handler) CreateUser(c *gin.Context) {
	var in userInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Password == "" {
		utils.Fail(c, http.StatusUnprocessableEntity, "password is required")
		return
	}
	var exists int64
	database.DB.Model(&models.User{}).Where("email = ?", in.Email).Count(&exists)
	if exists > 0 {
		utils.Fail(c, http.StatusBadRequest, "email is already registered")
		return
	}
	hash, _ := utils.HashPassword(in.Password)
	status := in.Status
	if status == "" {
		status = "active"
	}
	user := models.User{
		UUID: uuid.NewString(), Name: in.Name, Email: in.Email,
		Password: hash, Phone: in.Phone, Status: status,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create user")
		return
	}
	h.syncRoles(&user, in.RoleIDs)

	c.Set("activity", "created user "+user.Email)
	c.Set("activity_entity", "user")
	c.Set("activity_entity_id", strconv.Itoa(int(user.ID)))
	realtime.H.Broadcast("user.created", gin.H{"id": user.ID, "name": user.Name})
	utils.Created(c, user)
}

// UpdateUser handles PUT /api/v1/users/:id
func (h *Handler) UpdateUser(c *gin.Context) {
	var user models.User
	if err := database.DB.First(&user, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "user not found")
		return
	}
	var in userInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{"name": in.Name, "email": in.Email, "phone": in.Phone}
	if in.Status != "" {
		updates["status"] = in.Status
	}
	if in.Password != "" {
		hash, _ := utils.HashPassword(in.Password)
		updates["password"] = hash
	}
	if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not update user")
		return
	}
	h.syncRoles(&user, in.RoleIDs)

	c.Set("activity", "updated user "+user.Email)
	c.Set("activity_entity", "user")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, user)
}

// DeleteUser handles DELETE /api/v1/users/:id (soft delete).
func (h *Handler) DeleteUser(c *gin.Context) {
	me := middleware.CurrentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	if me != nil && me.ID == uint(id) {
		utils.Fail(c, http.StatusBadRequest, "you cannot delete your own account")
		return
	}
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "user not found")
		return
	}
	database.DB.Delete(&user)
	c.Set("activity", "deleted user "+user.Email)
	c.Set("activity_entity", "user")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "user deleted"})
}

func (h *Handler) syncRoles(user *models.User, roleIDs []uint) {
	if roleIDs == nil {
		return
	}
	var roles []models.Role
	if len(roleIDs) > 0 {
		database.DB.Find(&roles, roleIDs)
	}
	_ = database.DB.Model(user).Association("Roles").Replace(roles)
}
