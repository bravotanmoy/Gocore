package models

import "time"

// ActivityLog records every meaningful action performed through the CMS.
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `gorm:"index" json:"user_id"`
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Action    string    `gorm:"size:60;index" json:"action"`  // login, create, update, delete ...
	Entity    string    `gorm:"size:60;index" json:"entity"`  // user, post, product, order ...
	EntityID  string    `gorm:"size:40" json:"entity_id"`     // id of the affected record
	Detail    string    `gorm:"size:500" json:"detail"`       // human readable summary
	IP        string    `gorm:"size:45" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	Method    string    `gorm:"size:10" json:"method"`
	Path      string    `gorm:"size:255" json:"path"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// Setting is a key/value site setting grouped for the settings screen.
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"column:setting_key;size:100;uniqueIndex" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Group     string    `gorm:"size:60;default:general;index" json:"group"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ContactMessage is a submission from a site contact form.
type ContactMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:120" json:"name"`
	Email     string    `gorm:"size:190" json:"email"`
	Phone     string    `gorm:"size:32" json:"phone"`
	Subject   string    `gorm:"size:255" json:"subject"`
	Message   string    `gorm:"type:text" json:"message"`
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// Todo is a simple task item for the admin to-do screen.
type Todo struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index" json:"user_id"`
	Title     string     `gorm:"size:255" json:"title"`
	Done      bool       `gorm:"default:false" json:"done"`
	DueAt     *time.Time `json:"due_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Notification is delivered to a user in the admin header and over websocket.
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `gorm:"index" json:"user_id"` // nil = broadcast to everyone
	Title     string    `gorm:"size:255" json:"title"`
	Body      string    `gorm:"size:500" json:"body"`
	Type      string    `gorm:"size:30;default:info" json:"type"` // info | success | warning | error
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}
