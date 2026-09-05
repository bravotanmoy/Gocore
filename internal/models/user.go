package models

import (
	"time"

	"gorm.io/gorm"
)

// User is the account model for both CMS admins and website end-users.
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UUID      string         `gorm:"size:36;uniqueIndex" json:"uuid"`
	Name      string         `gorm:"size:120" json:"name"`
	Email     string         `gorm:"size:190;uniqueIndex" json:"email"`
	Password  string         `gorm:"size:255" json:"-"`
	Avatar    string         `gorm:"size:255" json:"avatar"`
	Phone     string         `gorm:"size:32" json:"phone"`
	Status    string         `gorm:"size:20;default:active" json:"status"` // active | blocked | pending
	LastLogin *time.Time     `json:"last_login"`
	Roles     []Role         `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Role groups permissions (e.g. super-admin, admin, editor, author, customer).
type Role struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	Name        string       `gorm:"size:60;uniqueIndex" json:"name"`
	DisplayName string       `gorm:"size:120" json:"display_name"`
	Description string       `gorm:"size:255" json:"description"`
	IsSystem    bool         `gorm:"default:false" json:"is_system"` // system roles can't be deleted
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Permission is a single ability, named "resource.action" (e.g. "content.create").
type Permission struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;uniqueIndex" json:"name"`
	Group     string    `gorm:"column:perm_group;size:60;index" json:"group"` // e.g. content, users, settings
	CreatedAt time.Time `json:"created_at"`
}

// RefreshToken stores issued refresh tokens so they can be revoked.
type RefreshToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Token     string    `gorm:"size:255;uniqueIndex" json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

// HasPermission checks (through preloaded roles) whether the user holds a permission.
func (u *User) HasPermission(perm string) bool {
	for _, r := range u.Roles {
		if r.Name == "super-admin" {
			return true
		}
		for _, p := range r.Permissions {
			if p.Name == perm {
				return true
			}
		}
	}
	return false
}

// HasRole checks whether the user holds a role by name.
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r.Name == role {
			return true
		}
	}
	return false
}
