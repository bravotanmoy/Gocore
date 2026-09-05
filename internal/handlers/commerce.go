package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------- Products ----------

// ListProducts handles GET /api/v1/products
func (h *Handler) ListProducts(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Product{}).Preload("Category")
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("name LIKE ? OR sku LIKE ?", like, like)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if cat := c.Query("category_id"); cat != "" {
		q = q.Where("category_id = ?", cat)
	}
	var total int64
	q.Count(&total)
	var products []models.Product
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&products)
	utils.Paginated(c, products, total, page, perPage)
}

// GetProduct handles GET /api/v1/products/:id
func (h *Handler) GetProduct(c *gin.Context) {
	var p models.Product
	if err := database.DB.Preload("Category").First(&p, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "product not found")
		return
	}
	utils.OK(c, p)
}

type productInput struct {
	Name        string   `json:"name" form:"name" binding:"required"`
	SKU         string   `json:"sku" form:"sku"`
	Description string   `json:"description" form:"description"`
	Price       float64  `json:"price" form:"price"`
	SalePrice   *float64 `json:"sale_price" form:"sale_price"`
	Stock       int      `json:"stock" form:"stock"`
	Image       string   `json:"image" form:"image"`
	Status      string   `json:"status" form:"status"`
	CategoryID  *uint    `json:"category_id" form:"category_id"`
}

// CreateProduct handles POST /api/v1/products
func (h *Handler) CreateProduct(c *gin.Context) {
	var in productInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	p := models.Product{
		Name: in.Name, Slug: uniqueSlug("products", utils.Slugify(in.Name), 0),
		SKU: in.SKU, Description: in.Description, Price: in.Price, SalePrice: in.SalePrice,
		Stock: in.Stock, Image: in.Image, Status: status, CategoryID: in.CategoryID,
	}
	if err := database.DB.Create(&p).Error; err != nil {
		utils.Fail(c, http.StatusInternalServerError, "could not create product")
		return
	}
	c.Set("activity", "created product "+p.Name)
	c.Set("activity_entity", "product")
	c.Set("activity_entity_id", strconv.Itoa(int(p.ID)))
	utils.Created(c, p)
}

// UpdateProduct handles PUT /api/v1/products/:id
func (h *Handler) UpdateProduct(c *gin.Context) {
	var p models.Product
	if err := database.DB.First(&p, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "product not found")
		return
	}
	var in productInput
	if err := c.ShouldBind(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updates := map[string]interface{}{
		"name": in.Name, "sku": in.SKU, "description": in.Description,
		"price": in.Price, "sale_price": in.SalePrice, "stock": in.Stock,
		"image": in.Image, "category_id": in.CategoryID,
	}
	if in.Status != "" {
		updates["status"] = in.Status
	}
	if in.Name != p.Name {
		updates["slug"] = uniqueSlug("products", utils.Slugify(in.Name), p.ID)
	}
	database.DB.Model(&p).Updates(updates)
	c.Set("activity", "updated product "+p.Name)
	c.Set("activity_entity", "product")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, p)
}

// DeleteProduct handles DELETE /api/v1/products/:id
func (h *Handler) DeleteProduct(c *gin.Context) {
	var p models.Product
	if err := database.DB.First(&p, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "product not found")
		return
	}
	database.DB.Delete(&p)
	c.Set("activity", "deleted product "+p.Name)
	c.Set("activity_entity", "product")
	c.Set("activity_entity_id", c.Param("id"))
	utils.OK(c, gin.H{"message": "product deleted"})
}

// ---------- Orders ----------

// ListOrders handles GET /api/v1/orders
func (h *Handler) ListOrders(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Order{}).Preload("Items").Preload("User")
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("order_number LIKE ? OR customer_name LIKE ? OR customer_email LIKE ?", like, like, like)
	}
	var total int64
	q.Count(&total)
	var orders []models.Order
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&orders)
	utils.Paginated(c, orders, total, page, perPage)
}

// GetOrder handles GET /api/v1/orders/:id
func (h *Handler) GetOrder(c *gin.Context) {
	var o models.Order
	if err := database.DB.Preload("Items.Product").Preload("User").First(&o, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "order not found")
		return
	}
	utils.OK(c, o)
}

type orderItemInput struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
}

type orderInput struct {
	CustomerName  string           `json:"customer_name" binding:"required"`
	CustomerEmail string           `json:"customer_email" binding:"required,email"`
	Phone         string           `json:"phone"`
	Address       string           `json:"address"`
	Shipping      float64          `json:"shipping"`
	Tax           float64          `json:"tax"`
	Items         []orderItemInput `json:"items" binding:"required,min=1"`
}

// buildOrder validates stock, prices the order server-side and stores it.
func (h *Handler) buildOrder(in orderInput, userID *uint) (*models.Order, string) {
	var items []models.OrderItem
	var subtotal float64
	for _, it := range in.Items {
		var p models.Product
		if err := database.DB.First(&p, it.ProductID).Error; err != nil {
			return nil, fmt.Sprintf("product %d not found", it.ProductID)
		}
		if p.Status != "active" {
			return nil, fmt.Sprintf("product %q is not available", p.Name)
		}
		if p.Stock < it.Quantity {
			return nil, fmt.Sprintf("insufficient stock for %q (%d left)", p.Name, p.Stock)
		}
		price := p.Price
		if p.SalePrice != nil && *p.SalePrice > 0 {
			price = *p.SalePrice
		}
		items = append(items, models.OrderItem{
			ProductID: p.ID, Name: p.Name, Price: price, Quantity: it.Quantity,
		})
		subtotal += price * float64(it.Quantity)
	}

	currency := "USD"
	var cur models.Setting
	if database.DB.Where("setting_key = ?", "currency").First(&cur).Error == nil && cur.Value != "" {
		currency = cur.Value
	}

	order := models.Order{
		OrderNumber:   fmt.Sprintf("ORD-%d", time.Now().UnixNano()/1e6),
		UserID:        userID,
		CustomerName:  in.CustomerName,
		CustomerEmail: in.CustomerEmail,
		Phone:         in.Phone,
		Address:       in.Address,
		Subtotal:      subtotal,
		Shipping:      in.Shipping,
		Tax:           in.Tax,
		Total:         subtotal + in.Shipping + in.Tax,
		Currency:      currency,
		Status:        "pending",
		Items:         items,
	}
	if err := database.DB.Create(&order).Error; err != nil {
		return nil, "could not create order"
	}
	// Decrement stock.
	for _, it := range items {
		database.DB.Model(&models.Product{}).Where("id = ?", it.ProductID).
			UpdateColumn("stock", gorm.Expr("stock - ?", it.Quantity))
	}
	realtime.H.Broadcast("order.created", gin.H{
		"id": order.ID, "order_number": order.OrderNumber,
		"total": order.Total, "customer": order.CustomerName,
	})
	return &order, ""
}

// CreateOrder handles POST /api/v1/orders (admin-created order).
func (h *Handler) CreateOrder(c *gin.Context) {
	var in orderInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	order, msg := h.buildOrder(in, nil)
	if msg != "" {
		utils.Fail(c, http.StatusBadRequest, msg)
		return
	}
	c.Set("activity", "created order "+order.OrderNumber)
	c.Set("activity_entity", "order")
	c.Set("activity_entity_id", strconv.Itoa(int(order.ID)))
	utils.Created(c, order)
}

var validOrderStatus = map[string]bool{
	"pending": true, "paid": true, "processing": true, "shipped": true,
	"completed": true, "cancelled": true, "refunded": true,
}

// UpdateOrderStatus handles PUT /api/v1/orders/:id/status {status}
func (h *Handler) UpdateOrderStatus(c *gin.Context) {
	var o models.Order
	if err := database.DB.First(&o, c.Param("id")).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "order not found")
		return
	}
	var in struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !validOrderStatus[in.Status] {
		utils.Fail(c, http.StatusUnprocessableEntity, "invalid status")
		return
	}
	database.DB.Model(&o).Update("status", in.Status)
	c.Set("activity", "order "+o.OrderNumber+" -> "+in.Status)
	c.Set("activity_entity", "order")
	c.Set("activity_entity_id", c.Param("id"))
	realtime.H.Broadcast("order.updated", gin.H{"id": o.ID, "order_number": o.OrderNumber, "status": in.Status})
	utils.OK(c, o)
}

// ListTransactions handles GET /api/v1/transactions
func (h *Handler) ListTransactions(c *gin.Context) {
	page, perPage, offset := utils.Pagination(c)
	q := database.DB.Model(&models.Transaction{}).Preload("Order")
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if g := c.Query("gateway"); g != "" {
		q = q.Where("gateway = ?", g)
	}
	var total int64
	q.Count(&total)
	var txns []models.Transaction
	q.Order("id DESC").Limit(perPage).Offset(offset).Find(&txns)
	utils.Paginated(c, txns, total, page, perPage)
}
