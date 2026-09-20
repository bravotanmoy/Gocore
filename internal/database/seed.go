package database

import (
	"log"

	"Gocorecms/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// permissionCatalog lists every permission the CMS knows about, grouped for the UI.
var permissionCatalog = map[string][]string{
	"users":    {"users.view", "users.create", "users.update", "users.delete"},
	"roles":    {"roles.view", "roles.create", "roles.update", "roles.delete"},
	"content":  {"content.view", "content.create", "content.update", "content.delete", "content.publish"},
	"media":    {"media.view", "media.upload", "media.delete"},
	"commerce": {"commerce.view", "commerce.manage_products", "commerce.manage_orders"},
	"settings": {"settings.view", "settings.update"},
	"activity": {"activity.view"},
	"comments": {"comments.view", "comments.moderate"},
}

// rolePresets maps default roles to their permissions. super-admin implicitly has all.
var rolePresets = map[string]struct {
	Display string
	Desc    string
	Perms   []string
}{
	"super-admin": {"Super Admin", "Full unrestricted access", nil},
	"admin": {"Admin", "Manage everything except roles & settings destruction", []string{
		"users.view", "users.create", "users.update", "users.delete",
		"roles.view",
		"content.view", "content.create", "content.update", "content.delete", "content.publish",
		"media.view", "media.upload", "media.delete",
		"commerce.view", "commerce.manage_products", "commerce.manage_orders",
		"settings.view", "settings.update",
		"activity.view",
		"comments.view", "comments.moderate",
	}},
	"editor": {"Editor", "Create and publish content", []string{
		"content.view", "content.create", "content.update", "content.publish",
		"media.view", "media.upload",
		"comments.view", "comments.moderate",
	}},
	"author": {"Author", "Write content, cannot publish", []string{
		"content.view", "content.create", "content.update",
		"media.view", "media.upload",
	}},
	"customer": {"Customer", "End user of the website / store", nil},
}

// Seed inserts permissions, default roles, the first super-admin and starter settings.
// It is idempotent — safe to run on every boot.
func Seed() error {
	// 1. Permissions
	perms := map[string]models.Permission{}
	for group, names := range permissionCatalog {
		for _, name := range names {
			var p models.Permission
			if err := DB.Where(models.Permission{Name: name}).
				Attrs(models.Permission{Group: group}).
				FirstOrCreate(&p).Error; err != nil {
				return err
			}
			perms[name] = p
		}
	}

	// 2. Roles with their permission sets
	for name, preset := range rolePresets {
		var role models.Role
		if err := DB.Where(models.Role{Name: name}).
			Attrs(models.Role{DisplayName: preset.Display, Description: preset.Desc, IsSystem: true}).
			FirstOrCreate(&role).Error; err != nil {
			return err
		}
		if len(preset.Perms) > 0 {
			var attach []models.Permission
			for _, pn := range preset.Perms {
				if p, ok := perms[pn]; ok {
					attach = append(attach, p)
				}
			}
			if err := DB.Model(&role).Association("Permissions").Replace(attach); err != nil {
				return err
			}
		}
	}

	// 3. First super-admin (admin@Gocore.com / admin123) — change after first login!
	var count int64
	DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			UUID:     uuid.NewString(),
			Name:     "Super Admin",
			Email:    "admin@Gocore.com",
			Password: string(hash),
			Status:   "active",
		}
		if err := DB.Create(&admin).Error; err != nil {
			return err
		}
		var superRole models.Role
		DB.Where("name = ?", "super-admin").First(&superRole)
		if err := DB.Model(&admin).Association("Roles").Append(&superRole); err != nil {
			return err
		}
		log.Println("seeded default super admin: admin@Gocore.com / admin123 (change this password!)")
	}

	// 4. Starter settings
	defaults := []models.Setting{
		{Key: "site_name", Value: "Gocore CMS", Group: "general"},
		{Key: "site_tagline", Value: "One CMS for every kind of website", Group: "general"},
		{Key: "site_url", Value: "http://localhost:8080", Group: "general"},
		{Key: "admin_email", Value: "admin@Gocore.com", Group: "general"},
		{Key: "posts_per_page", Value: "10", Group: "content"},
		{Key: "allow_registration", Value: "true", Group: "auth"},
		{Key: "default_role", Value: "customer", Group: "auth"},
		{Key: "currency", Value: "USD", Group: "commerce"},
		{Key: "maintenance_mode", Value: "false", Group: "general"},
	}
	for _, s := range defaults {
		var existing models.Setting
		if err := DB.Where(models.Setting{Key: s.Key}).Attrs(s).FirstOrCreate(&existing).Error; err != nil {
			return err
		}
	}

	// 5. Default menus
	for _, name := range []string{"main", "footer"} {
		var m models.Menu
		if err := DB.Where(models.Menu{Name: name}).FirstOrCreate(&m).Error; err != nil {
			return err
		}
	}

	// 6. Site content structure (categories, products, pages, menus)
	return seedYadeaContent()
}
