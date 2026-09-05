package middleware

import (
	"net/http"
	"strings"

	"Gocorecms/internal/config"
	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// CookieName is the admin panel session cookie holding the access JWT.
const CookieName = "Gocore_token"

// loadUser validates a JWT and loads the user (with roles+permissions) into the context.
func loadUser(c *gin.Context, cfg *config.Config, token string) bool {
	claims, err := utils.ParseAccessToken(cfg.JWTSecret, token)
	if err != nil {
		return false
	}
	var user models.User
	if err := database.DB.Preload("Roles.Permissions").First(&user, claims.UserID).Error; err != nil {
		return false
	}
	if user.Status != "active" {
		return false
	}
	c.Set("user", &user)
	return true
}

// AuthAPI protects JSON API routes with a Bearer token.
func AuthAPI(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token := ""
		if strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		} else if t, err := c.Cookie(CookieName); err == nil {
			token = t // allow same-origin cookie use of the API (admin panel JS)
		}
		if token == "" || !loadUser(c, cfg, token) {
			utils.Fail(c, http.StatusUnauthorized, "unauthenticated")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AuthWeb protects server-rendered admin pages; redirects browsers to /login.
func AuthWeb(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil || !loadUser(c, cfg, token) {
			c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentUser fetches the authenticated user placed in the context by AuthAPI/AuthWeb.
func CurrentUser(c *gin.Context) *models.User {
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*models.User); ok {
			return u
		}
	}
	return nil
}

// RequirePermission aborts unless the current user holds the permission (super-admin passes all).
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil || !user.HasPermission(perm) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				utils.Fail(c, http.StatusForbidden, "permission denied: "+perm)
			} else {
				c.HTML(http.StatusForbidden, "cms-403.html", gin.H{"title": "Forbidden", "permission": perm, "user": user})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}
