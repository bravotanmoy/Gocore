package models

import (
	"time"

	"gorm.io/gorm"
)

// Product is a sellable item for e-commerce sites.
type Product struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255" json:"name"`
	Slug        string         `gorm:"size:280;uniqueIndex" json:"slug"`
	SKU         string         `gorm:"size:80;index" json:"sku"`
	Description string         `gorm:"type:text" json:"description"`
	Price       float64        `gorm:"type:decimal(12,2)" json:"price"`
	SalePrice   *float64       `gorm:"type:decimal(12,2)" json:"sale_price"`
	Stock       int            `gorm:"default:0" json:"stock"`
	Image       string         `gorm:"size:255" json:"image"`
	Status      string         `gorm:"size:20;default:active;index" json:"status"` // active | draft | archived
	CategoryID  *uint          `json:"category_id"`
	Category    *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Order is a customer purchase.
// Status: pending | paid | processing | shipped | completed | cancelled | refunded
type Order struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	OrderNumber   string         `gorm:"size:40;uniqueIndex" json:"order_number"`
	UserID        *uint          `gorm:"index" json:"user_id"`
	User          *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CustomerName  string         `gorm:"size:120" json:"customer_name"`
	CustomerEmail string         `gorm:"size:190" json:"customer_email"`
	Phone         string         `gorm:"size:32" json:"phone"`
	Address       string         `gorm:"size:500" json:"address"`
	Subtotal      float64        `gorm:"type:decimal(12,2)" json:"subtotal"`
	Shipping      float64        `gorm:"type:decimal(12,2)" json:"shipping"`
	Tax           float64        `gorm:"type:decimal(12,2)" json:"tax"`
	Total         float64        `gorm:"type:decimal(12,2)" json:"total"`
	Currency      string         `gorm:"size:10;default:USD" json:"currency"`
	Status        string         `gorm:"size:20;default:pending;index" json:"status"`
	Items         []OrderItem    `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// OrderItem is one product line in an order (price is captured at order time).
type OrderItem struct {
	ID        uint     `gorm:"primaryKey" json:"id"`
	OrderID   uint     `gorm:"index" json:"order_id"`
	ProductID uint     `gorm:"index" json:"product_id"`
	Product   *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Name      string   `gorm:"size:255" json:"name"`
	Price     float64  `gorm:"type:decimal(12,2)" json:"price"`
	Quantity  int      `gorm:"default:1" json:"quantity"`
}

// Transaction records a payment attempt against an order.
// Gateway: stripe | sslcommerz | manual. Status: initiated | success | failed | refunded.
type Transaction struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderID    uint      `gorm:"index" json:"order_id"`
	Order      *Order    `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	Gateway    string    `gorm:"size:30;index" json:"gateway"`
	Reference  string    `gorm:"size:190;index" json:"reference"` // gateway session/txn id
	Amount     float64   `gorm:"type:decimal(12,2)" json:"amount"`
	Currency   string    `gorm:"size:10;default:USD" json:"currency"`
	Status     string    `gorm:"size:20;default:initiated;index" json:"status"`
	RawPayload string    `gorm:"type:text" json:"-"` // gateway response for audits
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
