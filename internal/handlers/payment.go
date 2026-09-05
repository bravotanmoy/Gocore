package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// InitiatePayment handles POST /api/v1/payments/initiate
// {order_id, gateway: "stripe"|"sslcommerz", success_url, cancel_url}
// Returns a checkout/redirect URL from the gateway.
func (h *Handler) InitiatePayment(c *gin.Context) {
	var in struct {
		OrderID    uint   `json:"order_id" binding:"required"`
		Gateway    string `json:"gateway" binding:"required,oneof=stripe sslcommerz"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.Fail(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var order models.Order
	if err := database.DB.Preload("Items").First(&order, in.OrderID).Error; err != nil {
		utils.Fail(c, http.StatusNotFound, "order not found")
		return
	}
	if order.Status != "pending" {
		utils.Fail(c, http.StatusBadRequest, "order is not payable (status: "+order.Status+")")
		return
	}
	if in.SuccessURL == "" {
		in.SuccessURL = h.Cfg.AppURL + "/payment/success"
	}
	if in.CancelURL == "" {
		in.CancelURL = h.Cfg.AppURL + "/payment/cancel"
	}

	var checkoutURL, reference, raw string
	var err error
	switch in.Gateway {
	case "stripe":
		checkoutURL, reference, raw, err = h.stripeCheckout(&order, in.SuccessURL, in.CancelURL)
	case "sslcommerz":
		checkoutURL, reference, raw, err = h.sslcommerzSession(&order, in.SuccessURL, in.CancelURL)
	}
	if err != nil {
		utils.Fail(c, http.StatusBadGateway, err.Error())
		return
	}

	txn := models.Transaction{
		OrderID: order.ID, Gateway: in.Gateway, Reference: reference,
		Amount: order.Total, Currency: order.Currency, Status: "initiated", RawPayload: raw,
	}
	database.DB.Create(&txn)
	c.Set("activity", "initiated "+in.Gateway+" payment for "+order.OrderNumber)
	c.Set("activity_entity", "transaction")
	c.Set("activity_entity_id", strconv.Itoa(int(txn.ID)))
	utils.OK(c, gin.H{"checkout_url": checkoutURL, "transaction_id": txn.ID, "reference": reference})
}

// stripeCheckout creates a Stripe Checkout Session via the REST API (no SDK needed).
func (h *Handler) stripeCheckout(order *models.Order, successURL, cancelURL string) (string, string, string, error) {
	if h.Cfg.StripeSecretKey == "" {
		return "", "", "", fmt.Errorf("stripe is not configured (set STRIPE_SECRET_KEY)")
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("client_reference_id", order.OrderNumber)
	form.Set("customer_email", order.CustomerEmail)
	for i, it := range order.Items {
		p := fmt.Sprintf("line_items[%d]", i)
		form.Set(p+"[quantity]", strconv.Itoa(it.Quantity))
		form.Set(p+"[price_data][currency]", strings.ToLower(order.Currency))
		form.Set(p+"[price_data][unit_amount]", strconv.Itoa(int(it.Price*100)))
		form.Set(p+"[price_data][product_data][name]", it.Name)
	}
	req, _ := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Bearer "+h.Cfg.StripeSecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("stripe request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &data); err != nil || data.Error != nil || data.URL == "" {
		msg := "unexpected stripe response"
		if data.Error != nil {
			msg = data.Error.Message
		}
		return "", "", string(body), fmt.Errorf("stripe: %s", msg)
	}
	return data.URL, data.ID, string(body), nil
}

// sslcommerzSession creates an SSLCommerz payment session.
func (h *Handler) sslcommerzSession(order *models.Order, successURL, cancelURL string) (string, string, string, error) {
	if h.Cfg.SSLCommerzStoreID == "" || h.Cfg.SSLCommerzStorePass == "" {
		return "", "", "", fmt.Errorf("sslcommerz is not configured (set SSLCOMMERZ_STORE_ID / SSLCOMMERZ_STORE_PASSWORD)")
	}
	endpoint := "https://securepay.sslcommerz.com/gwprocess/v4/api.php"
	if h.Cfg.SSLCommerzSandbox {
		endpoint = "https://sandbox.sslcommerz.com/gwprocess/v4/api.php"
	}
	form := url.Values{}
	form.Set("store_id", h.Cfg.SSLCommerzStoreID)
	form.Set("store_passwd", h.Cfg.SSLCommerzStorePass)
	form.Set("total_amount", fmt.Sprintf("%.2f", order.Total))
	form.Set("currency", order.Currency)
	form.Set("tran_id", order.OrderNumber)
	form.Set("success_url", successURL)
	form.Set("fail_url", cancelURL)
	form.Set("cancel_url", cancelURL)
	form.Set("ipn_url", h.Cfg.AppURL+"/api/v1/payments/webhook/sslcommerz")
	form.Set("cus_name", order.CustomerName)
	form.Set("cus_email", order.CustomerEmail)
	form.Set("cus_phone", order.Phone)
	form.Set("cus_add1", order.Address)
	form.Set("cus_city", "-")
	form.Set("cus_country", "Bangladesh")
	form.Set("shipping_method", "NO")
	form.Set("product_name", fmt.Sprintf("Order %s", order.OrderNumber))
	form.Set("product_category", "general")
	form.Set("product_profile", "general")

	resp, err := httpClient.PostForm(endpoint, form)
	if err != nil {
		return "", "", "", fmt.Errorf("sslcommerz request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Status     string `json:"status"`
		SessionKey string `json:"sessionkey"`
		GatewayURL string `json:"GatewayPageURL"`
		Reason     string `json:"failedreason"`
	}
	if err := json.Unmarshal(body, &data); err != nil || data.Status != "SUCCESS" || data.GatewayURL == "" {
		msg := data.Reason
		if msg == "" {
			msg = "unexpected sslcommerz response"
		}
		return "", "", string(body), fmt.Errorf("sslcommerz: %s", msg)
	}
	return data.GatewayURL, data.SessionKey, string(body), nil
}

// markPaid flips the transaction + order to paid and notifies over websocket.
func markPaid(txn *models.Transaction, raw string) {
	database.DB.Model(txn).Updates(map[string]interface{}{"status": "success", "raw_payload": raw})
	database.DB.Model(&models.Order{}).Where("id = ?", txn.OrderID).Update("status", "paid")
	var order models.Order
	database.DB.First(&order, txn.OrderID)
	realtime.H.Broadcast("payment.success", gin.H{
		"order_id": txn.OrderID, "order_number": order.OrderNumber, "amount": txn.Amount,
	})
}

// StripeWebhook handles POST /api/v1/payments/webhook/stripe.
// Note: for production set STRIPE_WEBHOOK_SECRET and verify the Stripe-Signature header.
func (h *Handler) StripeWebhook(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if event.Type == "checkout.session.completed" {
		var txn models.Transaction
		if database.DB.Where("reference = ? AND gateway = ?", event.Data.Object.ID, "stripe").First(&txn).Error == nil {
			markPaid(&txn, string(body))
		}
	}
	c.Status(http.StatusOK)
}

// SSLCommerzWebhook handles the SSLCommerz IPN POST.
// Note: for production validate against the SSLCommerz validation API before trusting.
func (h *Handler) SSLCommerzWebhook(c *gin.Context) {
	tranID := c.PostForm("tran_id")
	status := c.PostForm("status")
	if tranID == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	var order models.Order
	if err := database.DB.Where("order_number = ?", tranID).First(&order).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	var txn models.Transaction
	if err := database.DB.Where("order_id = ? AND gateway = ?", order.ID, "sslcommerz").
		Order("id DESC").First(&txn).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if status == "VALID" || status == "VALIDATED" {
		markPaid(&txn, c.Request.PostForm.Encode())
	} else {
		database.DB.Model(&txn).Update("status", "failed")
	}
	c.Status(http.StatusOK)
}
