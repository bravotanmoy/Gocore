package handlers

import (
	"net/http"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type registerInput struct {
	Name     string `json:"name" form:"name" binding:"required,min=2"`
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required,min=6"`
}

type loginInput struct {
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required"`
}

// issueTokens creates an access JWT + persisted refresh token for a user.
func (h *Handler) issueTokens(user *models.User) (access, refresh string, err error) {
	access, err = utils.GenerateAccessToken(h.Cfg.JWTSecret, user.ID, user.Email, time.Duration(h.Cfg.AccessTTLMin)*time.Minute)
	if err != nil {
		return
	}
	refresh, err = utils.GenerateRefreshToken()
	if err != nil {
		return
	}
	err = database.DB.Create(&models.RefreshToken{
		UserID:    user.ID,
		Token:     refresh,
		ExpiresAt: time.Now().Add(time.Duration(h.Cfg.RefreshTTLHours) * time.Hour),
	}).Error
	return
}

func (h *Handler) registerUser(in registerInput) (*models.User, string) {
	var allow models.Setting
	database.DB.Where("setting_key = ?", "allow_registration").First(&allow)
	if allow.Value == "false" {
		return nil, "registration is disabled"
	}

	var exists int64
	database.DB.Model(&models.User{}).Where("email = ?", in.Email).Count(&exists)
	if exists > 0 {
		return nil, "email is already registered"
	}

	hash, err := utils.HashPassword(in.Password)
	if err != nil {
		return nil, "could not hash password"
	}
	user := models.User{
		UUID:     uuid.NewString(),
		Name:     in.Name,
		Email:    in.Email,
		Password: hash,
		Status:   "active",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, "could not create user"
	}

	roleName := "customer"
	var def models.Setting
	if database.DB.Where("setting_key = ?", "default_role").First(&def).Error == nil && def.Value != "" {
		roleName = def.Value
	}
	var role models.Role
	if database.DB.Where("name = ?", roleName).First(&role).Error == nil {
		_ = database.DB.Model(&user).Association("Roles").Append(&role)
	}

	realtime.H.Broadcast("user.registered", gin.H{"id": user.ID, "name": user.Name})
	return &user, ""
}

func (h *Handler) authenticate(in loginInput, c *gin.Context) (*models.User, string) {
	var user models.User
	if err := database.DB.Preload("Roles.Permissions").Where("email = ?", in.Email).First(&user).Error; err != nil {
		return nil, "invalid email or password"
	}
	if !utils.CheckPassword(user.Password, in.Password) {
		return nil, "invalid email or password"
	}
	if user.Status != "active" {
		return nil, "account is " + user.Status
	}
	now := time.Now()
	database.DB.Model(&user).Update("last_login", &now)

	go func(uid uint, ip, ua, path string) {
		_ = database.DB.Create(&models.ActivityLog{
			UserID: &uid, Action: "login", Entity: "auth",
			Detail: "user logged in", IP: ip, UserAgent: ua, Method: "POST", Path: path,
		}).Error
	}(user.ID, c.ClientIP(), c.Request.UserAgent(), c.Request.URL.Path)

	return &user, ""
}

// ---------- JSON API ----------

// Register handles POST /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var in registerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user, msg := h.registerUser(in)
	if msg != "" {
		utils.Fail(c, http.StatusBadRequest, msg)
		return
	}
	access, refresh, err := h.issueTokens(user)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	utils.Created(c, gin.H{"user": user, "access_token": access, "refresh_token": refresh})
}

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	user, msg := h.authenticate(in, c)
	if msg != "" {
		utils.Fail(c, http.StatusUnauthorized, msg)
		return
	}
	access, refresh, err := h.issueTokens(user)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	utils.OK(c, gin.H{"user": user, "access_token": access, "refresh_token": refresh})
}

// Refresh handles POST /api/v1/auth/refresh {refresh_token}
func (h *Handler) Refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var rt models.RefreshToken
	if err := database.DB.Where("token = ? AND revoked = false", in.RefreshToken).First(&rt).Error; err != nil {
		utils.Fail(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if time.Now().After(rt.ExpiresAt) {
		utils.Fail(c, http.StatusUnauthorized, "refresh token expired")
		return
	}
	var user models.User
	if err := database.DB.First(&user, rt.UserID).Error; err != nil || user.Status != "active" {
		utils.Fail(c, http.StatusUnauthorized, "account unavailable")
		return
	}
	// Rotate: revoke old, issue new pair.
	database.DB.Model(&rt).Update("revoked", true)
	access, refresh, err := h.issueTokens(&user)
	if err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	utils.OK(c, gin.H{"access_token": access, "refresh_token": refresh})
}

// Logout handles POST /api/v1/auth/logout — revokes the given refresh token.
func (h *Handler) Logout(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&in)
	if in.RefreshToken != "" {
		database.DB.Model(&models.RefreshToken{}).Where("token = ?", in.RefreshToken).Update("revoked", true)
	}
	utils.OK(c, gin.H{"message": "logged out"})
}

// Me handles GET /api/v1/auth/me
func (h *Handler) Me(c *gin.Context) {
	utils.OK(c, middleware.CurrentUser(c))
}

// UpdateProfile handles PUT /api/v1/auth/me
func (h *Handler) UpdateProfile(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var in struct {
		Name   string `json:"name"`
		Phone  string `json:"phone"`
		Avatar string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{}
	if in.Name != "" {
		updates["name"] = in.Name
	}
	if in.Phone != "" {
		updates["phone"] = in.Phone
	}
	if in.Avatar != "" {
		updates["avatar"] = in.Avatar
	}
	if len(updates) > 0 {
		database.DB.Model(user).Updates(updates)
	}
	c.Set("activity", "updated own profile")
	utils.OK(c, user)
}

// ChangePassword handles PUT /api/v1/auth/password
func (h *Handler) ChangePassword(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var in struct {
		Current string `json:"current_password" binding:"required"`
		New     string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !utils.CheckPassword(user.Password, in.Current) {
		utils.Fail(c, http.StatusBadRequest, "current password is incorrect")
		return
	}
	hash, _ := utils.HashPassword(in.New)
	database.DB.Model(user).Update("password", hash)
	// Revoke all refresh tokens on password change.
	database.DB.Model(&models.RefreshToken{}).Where("user_id = ?", user.ID).Update("revoked", true)
	c.Set("activity", "changed own password")
	utils.OK(c, gin.H{"message": "password updated"})
}

// ---------- Admin panel (form posts, cookie session) ----------

func (h *Handler) setSessionCookie(c *gin.Context, token string) {
	secure := h.Cfg.AppEnv == "production"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.CookieName, token, h.Cfg.AccessTTLMin*60, "/", "", secure, true)
}

// WebLogin handles the admin login form POST.
func (h *Handler) WebLogin(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBind(&in); err != nil {
		c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login", "error": "Please enter a valid email and password."})
		return
	}
	user, msg := h.authenticate(in, c)
	if msg != "" {
		c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login", "error": msg, "email": in.Email})
		return
	}
	access, _, err := h.issueTokens(user)
	if err != nil {
		c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login", "error": "could not start session"})
		return
	}
	h.setSessionCookie(c, access)
	next := c.DefaultQuery("next", "/dashboard/")
	c.Redirect(http.StatusFound, next)
}

// WebRegister handles the admin register form POST.
func (h *Handler) WebRegister(c *gin.Context) {
	var in registerInput
	if err := c.ShouldBind(&in); err != nil {
		c.HTML(http.StatusOK, "register.html", gin.H{"title": "Register", "error": "All fields are required (password min 6 chars)."})
		return
	}
	user, msg := h.registerUser(in)
	if msg != "" {
		c.HTML(http.StatusOK, "register.html", gin.H{"title": "Register", "error": msg, "name": in.Name, "email": in.Email})
		return
	}
	access, _, err := h.issueTokens(user)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	h.setSessionCookie(c, access)
	c.Redirect(http.StatusFound, "/dashboard/")
}

// WebLogout clears the session cookie.
func (h *Handler) WebLogout(c *gin.Context) {
	c.SetCookie(middleware.CookieName, "", -1, "/", "", false, true)
	c.HTML(http.StatusOK, "logout.html", gin.H{"title": "Logout"})
}
