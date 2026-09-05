package database

import (
	"fmt"
	"log"

	"Gocorecms/internal/config"
	"Gocorecms/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global GORM handle used by handlers and middleware.
var DB *gorm.DB

// Connect opens the database using the configured driver (mysql default, postgres optional).
func Connect(cfg *config.Config) error {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DSN())
	case "mysql", "":
		dialector = mysql.Open(cfg.DSN())
	default:
		return fmt.Errorf("unsupported DB_DRIVER %q (use mysql or postgres)", cfg.DBDriver)
	}

	logLevel := logger.Warn
	if cfg.AppEnv == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("database connection failed (%s): %w", cfg.DBDriver, err)
	}

	DB = db
	log.Printf("connected to %s database %q on %s:%s", cfg.DBDriver, cfg.DBName, cfg.DBHost, cfg.DBPort)
	return nil
}

// Migrate creates/updates all CMS tables.
func Migrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.RefreshToken{},
		&models.Category{},
		&models.Tag{},
		&models.Post{},
		&models.Page{},
		&models.Media{},
		&models.Menu{},
		&models.MenuItem{},
		&models.Comment{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
		&models.Transaction{},
		&models.ActivityLog{},
		&models.Setting{},
		&models.ContactMessage{},
		&models.Todo{},
		&models.Notification{},
	)
}
