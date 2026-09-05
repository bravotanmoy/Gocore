package middleware

import (
	"strings"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"

	"github.com/gin-gonic/gin"
)

// ActivityLogger records every mutating request (POST/PUT/PATCH/DELETE) after it runs.
// Handlers can enrich the entry via c.Set("activity", "...") / c.Set("activity_entity", ...).
func ActivityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return
		}
		path := c.Request.URL.Path
		// Skip noise: static assets, websocket, and failed logins are logged by the auth handler itself.
		if strings.HasPrefix(path, "/assets") || strings.HasPrefix(path, "/uploads") || strings.HasPrefix(path, "/api/v1/ws") {
			return
		}
		if c.Writer.Status() >= 400 {
			return
		}

		entry := models.ActivityLog{
			Action:    strings.ToLower(actionFor(method)),
			Entity:    c.GetString("activity_entity"),
			EntityID:  c.GetString("activity_entity_id"),
			Detail:    c.GetString("activity"),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Method:    method,
			Path:      path,
		}
		if a := c.GetString("activity_action"); a != "" {
			entry.Action = a
		}
		if entry.Detail == "" {
			entry.Detail = method + " " + path
		}
		if entry.Entity == "" {
			entry.Entity = guessEntity(path)
		}
		if u := CurrentUser(c); u != nil {
			entry.UserID = &u.ID
		}
		// Fire and forget — logging must never slow down or fail a request.
		go func(e models.ActivityLog) {
			_ = database.DB.Create(&e).Error
		}(entry)
	}
}

func actionFor(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return method
}

// guessEntity extracts a resource name from paths like /api/v1/users/5 or /dashboard/cms/posts.
func guessEntity(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p == "" || isNumeric(p) {
			continue
		}
		return p
	}
	return "request"
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
