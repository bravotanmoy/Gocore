package models

import (
	"time"

	"gorm.io/gorm"
)

// Category organizes posts and products into a tree (ParentID nullable).
type Category struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:120" json:"name"`
	Slug        string     `gorm:"size:150;uniqueIndex" json:"slug"`
	Description string     `gorm:"size:255" json:"description"`
	Type        string     `gorm:"size:20;default:post;index" json:"type"` // post | product
	ParentID    *uint      `json:"parent_id"`
	Parent      *Category  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children    []Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Tag is a free-form label attached to posts.
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:80" json:"name"`
	Slug      string    `gorm:"size:100;uniqueIndex" json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// Post is a blog/news article. Status: draft | published | archived.
type Post struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Title         string         `gorm:"size:255" json:"title"`
	Slug          string         `gorm:"size:280;uniqueIndex" json:"slug"`
	Excerpt       string         `gorm:"size:500" json:"excerpt"`
	Body          string         `gorm:"type:text" json:"body"`
	FeaturedImage string         `gorm:"size:255" json:"featured_image"`
	Status        string         `gorm:"size:20;default:draft;index" json:"status"`
	AuthorID      uint           `gorm:"index" json:"author_id"`
	Author        *User          `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	CategoryID    *uint          `json:"category_id"`
	Category      *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Tags          []Tag          `gorm:"many2many:post_tags;" json:"tags,omitempty"`
	Views         uint           `gorm:"default:0" json:"views"`
	MetaTitle     string         `gorm:"size:255" json:"meta_title"`
	MetaDesc      string         `gorm:"size:500" json:"meta_description"`
	PublishedAt   *time.Time     `json:"published_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Page is a static site page (About, Landing sections, etc.).
// Body can hold HTML or a JSON block structure the frontend interprets.
type Page struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"size:255" json:"title"`
	Slug      string         `gorm:"size:280;uniqueIndex" json:"slug"`
	Body      string         `gorm:"type:text" json:"body"`
	Template  string         `gorm:"size:60;default:default" json:"template"`
	Status    string         `gorm:"size:20;default:draft;index" json:"status"`
	AuthorID  uint           `gorm:"index" json:"author_id"`
	Author    *User          `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	MetaTitle string         `gorm:"size:255" json:"meta_title"`
	MetaDesc  string         `gorm:"size:500" json:"meta_description"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Media is an uploaded file stored under the configured upload dir.
type Media struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FileName   string    `gorm:"size:255" json:"file_name"`
	Path       string    `gorm:"size:255" json:"path"` // public URL path e.g. /uploads/2026/09/x.png
	MimeType   string    `gorm:"size:120" json:"mime_type"`
	Size       int64     `json:"size"`
	UploadedBy uint      `gorm:"index" json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// Menu is a named navigation menu (e.g. "main", "footer").
type Menu struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Name      string     `gorm:"size:80;uniqueIndex" json:"name"`
	Items     []MenuItem `gorm:"foreignKey:MenuID" json:"items,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// MenuItem is one link in a menu; supports nesting via ParentID.
type MenuItem struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	MenuID    uint   `gorm:"index" json:"menu_id"`
	ParentID  *uint  `json:"parent_id"`
	Label     string `gorm:"size:120" json:"label"`
	URL       string `gorm:"size:255" json:"url"`
	Target    string `gorm:"size:20;default:_self" json:"target"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`
}

// Comment belongs to a post; Status: pending | approved | spam.
type Comment struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	PostID    uint           `gorm:"index" json:"post_id"`
	UserID    *uint          `json:"user_id"`
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Name      string         `gorm:"size:120" json:"name"`  // for guest comments
	Email     string         `gorm:"size:190" json:"email"` // for guest comments
	Body      string         `gorm:"type:text" json:"body"`
	Status    string         `gorm:"size:20;default:pending;index" json:"status"`
	ParentID  *uint          `json:"parent_id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
