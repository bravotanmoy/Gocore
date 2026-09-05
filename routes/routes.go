package routes

import (
	"net/http"

	"Gocorecms/internal/config"
	"Gocorecms/internal/handlers"
	"Gocorecms/internal/middleware"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
)

// page returns a handler that renders a plain template with a title.
func page(tmpl, title string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, tmpl, gin.H{
			"title": title,
			"user":  middleware.CurrentUser(c),
		})
	}
}

// SetupRoutes wires the public site, auth pages, admin panel and the JSON API.
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	h := handlers.New(cfg)

	// ---------- Public site pages ----------
	r.GET("/", page("landing.html", "Home"))
	r.GET("/features", page("features.html", "Features"))
	r.GET("/our-team", page("our-team.html", "Our Team"))
	r.GET("/faqs", page("faqs.html", "FAQ's"))
	r.GET("/contact", page("contact.html", "Contact"))

	// ---------- Auth pages (forms post to the same URL) ----------
	r.GET("/login", func(c *gin.Context) {
		if token, err := c.Cookie(middleware.CookieName); err == nil {
			if _, err := utils.ParseAccessToken(cfg.JWTSecret, token); err == nil {
				c.Redirect(http.StatusFound, "/dashboard/")
				return
			}
		}
		c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login"})
	})
	r.POST("/login", h.WebLogin)
	r.GET("/register", page("register.html", "Register"))
	r.POST("/register", h.WebRegister)
	r.GET("/logout", h.WebLogout)
	r.GET("/reset-password", page("reset-password.html", "Reset Password"))
	r.GET("/forget-password", page("forget-password.html", "Forget Password"))
	r.GET("/confirm-mail", page("confirm-mail.html", "Confirm Mail"))
	r.GET("/lock-screen", page("lock-screen.html", "Lock Screen"))

	// ---------- JSON API (for Nuxt / Next / plain HTML frontends) ----------
	api := r.Group("/api/v1")
	{
		// Auth
		api.POST("/auth/register", h.Register)
		api.POST("/auth/login", h.Login)
		api.POST("/auth/refresh", h.Refresh)
		api.POST("/auth/logout", h.Logout)

		// Realtime
		api.GET("/ws", h.WS)

		// Payment webhooks (called by the gateways, no auth)
		api.POST("/payments/webhook/stripe", h.StripeWebhook)
		api.POST("/payments/webhook/sslcommerz", h.SSLCommerzWebhook)

		// Public content for headless frontends
		pub := api.Group("/public")
		{
			pub.GET("/posts", h.PublicPosts)
			pub.GET("/posts/:slug", h.PublicPost)
			pub.POST("/posts/:slug/comments", h.PublicComment)
			pub.GET("/pages/:slug", h.PublicPage)
			pub.GET("/products", h.PublicProducts)
			pub.GET("/products/:slug", h.PublicProduct)
			pub.GET("/categories", h.PublicCategories)
			pub.GET("/menus/:name", h.PublicMenu)
			pub.GET("/settings", h.PublicSettings)
			pub.POST("/contact", h.PublicContact)
			pub.POST("/orders", h.PublicOrder)
		}

		// Authenticated API
		auth := api.Group("", middleware.AuthAPI(cfg))
		{
			auth.GET("/auth/me", h.Me)
			auth.PUT("/auth/me", h.UpdateProfile)
			auth.PUT("/auth/password", h.ChangePassword)

			auth.GET("/dashboard/stats", h.Stats)

			auth.GET("/users", middleware.RequirePermission("users.view"), h.ListUsers)
			auth.GET("/users/:id", middleware.RequirePermission("users.view"), h.GetUser)
			auth.POST("/users", middleware.RequirePermission("users.create"), h.CreateUser)
			auth.PUT("/users/:id", middleware.RequirePermission("users.update"), h.UpdateUser)
			auth.DELETE("/users/:id", middleware.RequirePermission("users.delete"), h.DeleteUser)

			auth.GET("/roles", middleware.RequirePermission("roles.view"), h.ListRoles)
			auth.GET("/roles/:id", middleware.RequirePermission("roles.view"), h.GetRole)
			auth.POST("/roles", middleware.RequirePermission("roles.create"), h.CreateRole)
			auth.PUT("/roles/:id", middleware.RequirePermission("roles.update"), h.UpdateRole)
			auth.DELETE("/roles/:id", middleware.RequirePermission("roles.delete"), h.DeleteRole)
			auth.GET("/permissions", middleware.RequirePermission("roles.view"), h.ListPermissions)

			auth.GET("/activity", middleware.RequirePermission("activity.view"), h.ListActivity)

			auth.GET("/posts", middleware.RequirePermission("content.view"), h.ListPosts)
			auth.GET("/posts/:id", middleware.RequirePermission("content.view"), h.GetPost)
			auth.POST("/posts", middleware.RequirePermission("content.create"), h.CreatePost)
			auth.PUT("/posts/:id", middleware.RequirePermission("content.update"), h.UpdatePost)
			auth.DELETE("/posts/:id", middleware.RequirePermission("content.delete"), h.DeletePost)

			auth.GET("/pages", middleware.RequirePermission("content.view"), h.ListPages)
			auth.POST("/pages", middleware.RequirePermission("content.create"), h.CreatePage)
			auth.PUT("/pages/:id", middleware.RequirePermission("content.update"), h.UpdatePage)
			auth.DELETE("/pages/:id", middleware.RequirePermission("content.delete"), h.DeletePage)

			auth.GET("/categories", middleware.RequirePermission("content.view"), h.ListCategories)
			auth.POST("/categories", middleware.RequirePermission("content.create"), h.CreateCategory)
			auth.PUT("/categories/:id", middleware.RequirePermission("content.update"), h.UpdateCategory)
			auth.DELETE("/categories/:id", middleware.RequirePermission("content.delete"), h.DeleteCategory)

			auth.GET("/comments", middleware.RequirePermission("comments.view"), h.ListComments)
			auth.PUT("/comments/:id", middleware.RequirePermission("comments.moderate"), h.ModerateComment)
			auth.DELETE("/comments/:id", middleware.RequirePermission("comments.moderate"), h.DeleteComment)

			auth.GET("/media", middleware.RequirePermission("media.view"), h.ListMedia)
			auth.POST("/media", middleware.RequirePermission("media.upload"), h.UploadMedia)
			auth.DELETE("/media/:id", middleware.RequirePermission("media.delete"), h.DeleteMedia)

			auth.GET("/products", middleware.RequirePermission("commerce.view"), h.ListProducts)
			auth.GET("/products/:id", middleware.RequirePermission("commerce.view"), h.GetProduct)
			auth.POST("/products", middleware.RequirePermission("commerce.manage_products"), h.CreateProduct)
			auth.PUT("/products/:id", middleware.RequirePermission("commerce.manage_products"), h.UpdateProduct)
			auth.DELETE("/products/:id", middleware.RequirePermission("commerce.manage_products"), h.DeleteProduct)

			auth.GET("/orders", middleware.RequirePermission("commerce.view"), h.ListOrders)
			auth.GET("/orders/:id", middleware.RequirePermission("commerce.view"), h.GetOrder)
			auth.POST("/orders", middleware.RequirePermission("commerce.manage_orders"), h.CreateOrder)
			auth.PUT("/orders/:id/status", middleware.RequirePermission("commerce.manage_orders"), h.UpdateOrderStatus)
			auth.GET("/transactions", middleware.RequirePermission("commerce.view"), h.ListTransactions)

			auth.POST("/payments/initiate", h.InitiatePayment)

			auth.GET("/settings", middleware.RequirePermission("settings.view"), h.ListSettings)
			auth.PUT("/settings", middleware.RequirePermission("settings.update"), h.UpdateSettings)

			auth.GET("/contact-messages", middleware.RequirePermission("settings.view"), h.ListContactMessages)
			auth.PUT("/contact-messages/:id/read", middleware.RequirePermission("settings.view"), h.MarkContactRead)
			auth.DELETE("/contact-messages/:id", middleware.RequirePermission("settings.update"), h.DeleteContactMessage)

			auth.GET("/todos", h.ListTodos)
			auth.POST("/todos", h.CreateTodo)
			auth.PUT("/todos/:id/toggle", h.ToggleTodo)
			auth.DELETE("/todos/:id", h.DeleteTodo)

			auth.GET("/notifications", h.ListNotifications)
			auth.POST("/notifications", middleware.RequirePermission("users.update"), h.SendNotification)
			auth.PUT("/notifications/:id/read", h.MarkNotificationRead)
		}
	}

	// ---------- Admin panel (session cookie required) ----------
	dashboard := r.Group("/dashboard", middleware.AuthWeb(cfg))
	{
		// Functional CMS screens
		cms := dashboard.Group("/cms")
		{
			cms.GET("", h.CMSDashboard)
			cms.GET("/", h.CMSDashboard)

			cms.GET("/users", middleware.RequirePermission("users.view"), h.CMSUsers)
			cms.GET("/users/new", middleware.RequirePermission("users.create"), h.CMSUserForm)
			cms.GET("/users/:id/edit", middleware.RequirePermission("users.update"), h.CMSUserForm)
			cms.POST("/users", middleware.RequirePermission("users.create"), h.CMSUserSave)
			cms.POST("/users/:id", middleware.RequirePermission("users.update"), h.CMSUserSave)
			cms.POST("/users/:id/delete", middleware.RequirePermission("users.delete"), h.CMSUserDelete)

			cms.GET("/roles", middleware.RequirePermission("roles.view"), h.CMSRoles)
			cms.GET("/roles/new", middleware.RequirePermission("roles.create"), h.CMSRoleForm)
			cms.GET("/roles/:id/edit", middleware.RequirePermission("roles.update"), h.CMSRoleForm)
			cms.POST("/roles", middleware.RequirePermission("roles.create"), h.CMSRoleSave)
			cms.POST("/roles/:id", middleware.RequirePermission("roles.update"), h.CMSRoleSave)
			cms.POST("/roles/:id/delete", middleware.RequirePermission("roles.delete"), h.CMSRoleDelete)

			cms.GET("/activity", middleware.RequirePermission("activity.view"), h.CMSActivity)

			cms.GET("/posts", middleware.RequirePermission("content.view"), h.CMSPosts)
			cms.GET("/posts/new", middleware.RequirePermission("content.create"), h.CMSPostForm)
			cms.GET("/posts/:id/edit", middleware.RequirePermission("content.update"), h.CMSPostForm)
			cms.POST("/posts", middleware.RequirePermission("content.create"), h.CMSPostSave)
			cms.POST("/posts/:id", middleware.RequirePermission("content.update"), h.CMSPostSave)
			cms.POST("/posts/:id/delete", middleware.RequirePermission("content.delete"), h.CMSPostDelete)

			cms.GET("/categories", middleware.RequirePermission("content.view"), h.CMSCategories)
			cms.POST("/categories", middleware.RequirePermission("content.create"), h.CMSCategorySave)
			cms.POST("/categories/:id/delete", middleware.RequirePermission("content.delete"), h.CMSCategoryDelete)

			cms.GET("/media", middleware.RequirePermission("media.view"), h.CMSMedia)
			cms.POST("/media", middleware.RequirePermission("media.upload"), h.CMSMediaUpload)
			cms.POST("/media/:id/delete", middleware.RequirePermission("media.delete"), h.CMSMediaDelete)

			cms.GET("/products", middleware.RequirePermission("commerce.view"), h.CMSProducts)
			cms.GET("/products/new", middleware.RequirePermission("commerce.manage_products"), h.CMSProductForm)
			cms.GET("/products/:id/edit", middleware.RequirePermission("commerce.manage_products"), h.CMSProductForm)
			cms.POST("/products", middleware.RequirePermission("commerce.manage_products"), h.CMSProductSave)
			cms.POST("/products/:id", middleware.RequirePermission("commerce.manage_products"), h.CMSProductSave)
			cms.POST("/products/:id/delete", middleware.RequirePermission("commerce.manage_products"), h.CMSProductDelete)

			cms.GET("/orders", middleware.RequirePermission("commerce.view"), h.CMSOrders)
			cms.POST("/orders/:id/status", middleware.RequirePermission("commerce.manage_orders"), h.CMSOrderStatus)

			cms.GET("/settings", middleware.RequirePermission("settings.view"), h.CMSSettings)
			cms.POST("/settings", middleware.RequirePermission("settings.update"), h.CMSSettingsSave)

			cms.GET("/profile", h.CMSProfile)
			cms.POST("/profile", h.CMSProfileSave)
		}

		// Demo dashboards shipped with the Gocore template
		dashboard.GET("/", page("ecommarce.html", "eCommerce"))
		dashboard.GET("/crm", page("crm.html", "CRM"))
		dashboard.GET("/project-management", page("project-management.html", "Project Management"))
		dashboard.GET("/lms", page("lms.html", "LMS"))
		dashboard.GET("/help-desk", page("help-desk.html", "Help Desk"))
		dashboard.GET("/analytics", page("analytics.html", "Analytics"))
		dashboard.GET("/crypto", page("crypto.html", "Crypto"))
		dashboard.GET("/sales", page("sales.html", "Sales"))
		dashboard.GET("/hospital", page("hospital.html", "Hospital"))
		dashboard.GET("/marketing", page("marketing.html", "Marketing"))
		dashboard.GET("/nft", page("nft.html", "NFT"))
		dashboard.GET("/saas", page("saas.html", "SaaS"))
		dashboard.GET("/real-estate", page("real-estate.html", "Real Estate"))
		dashboard.GET("/shipment", page("shipment.html", "Shipment"))
		dashboard.GET("/finance", page("finance.html", "Finance"))
		dashboard.GET("/hrm", page("hrm.html", "HRM"))
		dashboard.GET("/school", page("school.html", "School"))
		dashboard.GET("/call-center", page("call-center.html", "Call Center"))
		dashboard.GET("/pos-system", page("pos-system.html", "POS System"))
		dashboard.GET("/podcast", page("podcast.html", "Podcast"))
		dashboard.GET("/doctor", page("doctor.html", "Doctor"))
		dashboard.GET("/social-media", page("social-media.html", "Social Media"))
		dashboard.GET("/beauty-salon", page("beauty-salon.html", "Beauty Salon"))
		dashboard.GET("/store-analytics", page("store-analytics.html", "Store Analytics"))
		dashboard.GET("/restaurant", page("restaurant.html", "Restaurant"))
		dashboard.GET("/hotel", page("hotel.html", "Hotel"))
		dashboard.GET("/real-estate-agent", page("real-estate-agent.html", "Real Estate Agent"))
		dashboard.GET("/credit-card", page("credit-card.html", "Credit Card"))
		dashboard.GET("/crypto-trader", page("crypto-trader.html", "Crypto Trader"))
		dashboard.GET("/crypto-performance", page("crypto-performance.html", "Crypto Performance"))
		dashboard.GET("/to-do-list", page("to-do-list.html", "To Do List"))
		dashboard.GET("/calendar", page("calendar.html", "Calendar"))
		dashboard.GET("/contacts", page("contacts.html", "Contacts"))
		dashboard.GET("/chat", page("chat.html", "Chat"))
		dashboard.GET("/inbox", page("inbox.html", "Inbox"))
		dashboard.GET("/compose", page("compose.html", "Compose"))
		dashboard.GET("/read-email", page("read-email.html", "Read Email"))
		// Mail sub-views without dedicated templates reuse the inbox view.
		dashboard.GET("/snoozed", page("inbox.html", "Snoozed"))
		dashboard.GET("/draft", page("inbox.html", "Draft"))
		dashboard.GET("/sent-mail", page("inbox.html", "Sent Mail"))
		dashboard.GET("/trash-email", page("inbox.html", "Trash"))
		dashboard.GET("/spam", page("inbox.html", "Spam"))
		dashboard.GET("/starred", page("inbox.html", "Starred"))
		dashboard.GET("/important", page("inbox.html", "Important"))
	}
}
