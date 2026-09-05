package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"
	"time"

	"Gocorecms/internal/config"
	"Gocorecms/internal/database"
	"Gocorecms/internal/middleware"
	"Gocorecms/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// templateFuncs are helpers available inside all templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"fdate": func(v interface{}) string {
			const layout = "Jan 02, 2006 3:04 PM"
			switch t := v.(type) {
			case time.Time:
				return t.Format(layout)
			case *time.Time:
				if t != nil {
					return t.Format(layout)
				}
			}
			return ""
		},
		"fmoney": func(v interface{}) string {
			switch n := v.(type) {
			case float64:
				return fmt.Sprintf("$%.2f", n)
			case float32:
				return fmt.Sprintf("$%.2f", n)
			case int:
				return fmt.Sprintf("$%d", n)
			}
			return fmt.Sprintf("%v", v)
		},
		"fsize": func(size int64) string {
			switch {
			case size >= 1<<20:
				return fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
			case size >= 1<<10:
				return fmt.Sprintf("%.1f KB", float64(size)/(1<<10))
			}
			return fmt.Sprintf("%d B", size)
		},
		"deref":     func(p *uint) uint { if p != nil { return *p }; return 0 },
		"deref64":   func(p *float64) float64 { if p != nil { return *p }; return 0 },
		"hasPrefix": strings.HasPrefix,
	}
}

func main() {
	cfg := config.Load()

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Database: connect, migrate, seed defaults (roles, permissions, admin user).
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("FATAL: %v\nCheck your .env DB_* settings (driver=%s host=%s port=%s db=%s)",
			err, cfg.DBDriver, cfg.DBHost, cfg.DBPort, cfg.DBName)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("FATAL: migration failed: %v", err)
	}
	if err := database.Seed(); err != nil {
		log.Fatalf("FATAL: seeding failed: %v", err)
	}

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("FATAL: cannot create upload dir %s: %v", cfg.UploadDir, err)
	}

	r := gin.Default()
	r.MaxMultipartMemory = 16 << 20

	// CORS for headless frontends (Next/Nuxt/plain HTML on other origins).
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Record every mutating request in the activity log.
	r.Use(middleware.ActivityLogger())

	r.SetFuncMap(templateFuncs())
	r.Static("/assets", "./static")
	r.Static("/uploads", cfg.UploadDir)
	r.LoadHTMLGlob("templates/*.html")

	routes.SetupRoutes(r, cfg)

	log.Printf("%s listening on %s (env: %s, db: %s)", cfg.AppName, cfg.AppURL, cfg.AppEnv, cfg.DBDriver)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("FATAL: server stopped: %v", err)
	}
}
