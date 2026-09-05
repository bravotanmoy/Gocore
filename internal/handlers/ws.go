package handlers

import (
	"net/http"

	"Gocorecms/internal/database"
	"Gocorecms/internal/models"
	"Gocorecms/internal/realtime"
	"Gocorecms/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CORS for websockets: origins are already restricted by the browser +
	// token auth below; allow any origin so Nuxt/Next dev servers can connect.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WS handles GET /api/v1/ws?token=<access_token> — upgrades to a websocket that
// receives realtime events (notifications, orders, content changes...).
func (h *Handler) WS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		if t, err := c.Cookie("Gocore_token"); err == nil {
			token = t
		}
	}
	claims, err := utils.ParseAccessToken(h.Cfg.JWTSecret, token)
	if err != nil {
		utils.Fail(c, http.StatusUnauthorized, "invalid token")
		return
	}
	var user models.User
	if err := database.DB.First(&user, claims.UserID).Error; err != nil || user.Status != "active" {
		utils.Fail(c, http.StatusUnauthorized, "account unavailable")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &realtime.Client{UserID: user.ID, Conn: conn, Send: make(chan []byte, 32)}
	realtime.H.Register(client)

	// Reader loop: we ignore inbound messages but need it to detect disconnects.
	go func() {
		defer realtime.H.Unregister(client)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
