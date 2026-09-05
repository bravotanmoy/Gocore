package handlers

import (
	"net/http"
	"strconv"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListRoles handles GET /api/v1/roles
func (h *Handler) ListRoles(c *gin.Context) {
	var roles []models.Role
	database.DB.Preload("Permissions").Order("id").Find(&roles)
	utils.OK(c, roles)
}

// GetRole handles GET /api/v1/roles/:id
func (h *Handler) GetRole(c *gin.Context) {
	var role models.Role
	if err := database.DB.Preload("Permissions").First(&role, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "role not found")
		return
	}
	utils.OK(c, role)
}

type roleInput struct {
	Name          string `json:"name" form:"name" binding:"required"`
	DisplayName   string `json:"display_name" form:"display_name"`
	Description   string `json:"description" form:"description"`
	PermissionIDs []uint `json:"permission_ids" form:"permission_ids"`
}

// CreateRole handles POST /api/v1/roles
func (h *Handler) CreateRole(c *gin.Context) {
	var in roleInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	name := utils.Slugify(in.Name)
	var exists int64
	database.DB.Model(&models.Role{}).Where("name = ?", name).Count(&exists)
	if exists > 0 {
		utils.Fail(c, http.StatusBadRequest, "role already exists")
		return
	}
	display := in.DisplayName
	if display == "" {
		display = in.Name
	}
	role := models.Role{Name: name, DisplayName: display, Description: in.Description}
	if err := database.DB.Create(&role).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create role")
		return
	}
	h.syncPermissions(&role, in.PermissionIDs)

	c.Set("activity", "created role "+role.Name)
	c.Set("activity_entity", "role")
	c.Set("activity_entity_id", strconv.Itoa(int(role.ID)))
	utils.Created(c, role)
}

// UpdateRole handles PUT /api/v1/roles/:id
func (h *Handler) UpdateRole(c *gin.Context) {
	var role models.Role
	if err := database.DB.First(&role, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "role not found")
		return
	}
	var in roleInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{"description": in.Description}
	if in.DisplayName != "" {
		updates["display_name"] = in.DisplayName
	}
	// System role names are fixed; custom role names can change.
	if !role.IsSystem && in.Name != "" {
		updates["name"] = utils.Slugify(in.Name)
	}
	database.DB.Model(&role).Updates(updates)
	h.syncPermissions(&role, in.PermissionIDs)

	c.Set("activity", "updated role "+role.Name)
	c.Set("activity_entity", "role")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, role)
}

// DeleteRole handles DELETE /api/v1/roles/:id
func (h *Handler) DeleteRole(c *gin.Context) {
	var role models.Role
	if err := database.DB.First(&role, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "role not found")
		return
	}
	if role.IsSystem {
		utils.Fail(c, http.StatusBadRequest, "system roles cannot be deleted")
		return
	}
	_ = database.DB.Model(&role).Association("Permissions").Clear()
	database.DB.Exec("DELETE FROM user_roles WHERE role_id = ?", role.ID)
	database.DB.Delete(&role)

	c.Set("activity", "deleted role "+role.Name)
	c.Set("activity_entity", "role")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "role deleted"})
}

// ListPermissions handles GET /api/v1/permissions — grouped for the UI.
func (h *Handler) ListPermissions(c *gin.Context) {
	var perms []models.Permission
	database.DB.Order("perm_group, name").Find(&perms)
	grouped := map[string][]models.Permission{}
	for _, p := range perms {
		grouped[p.Group] = append(grouped[p.Group], p)
	}
	utils.OK(c, grouped)
}

func (h *Handler) syncPermissions(role *models.Role, ids []uint) {
	if ids == nil {
		return
	}
	var perms []models.Permission
	if len(ids) > 0 {
		database.DB.Find(&perms, ids)
	}
	_ = database.DB.Model(role).Association("Permissions").Replace(perms)
}
