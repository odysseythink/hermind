package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

const AuthDisabledBypassToken = "AUTH_DISABLED_BYPASS"

// WSValidatedRequest authenticates a WebSocket upgrade request via
// query string token (?token=allm-tat-...). The token is a single-use
// short-TTL temp token issued by POST /workspace/:slug/agent-token.
func WSValidatedRequest(authSvc *services.AuthService, tempTokenSvc *services.TemporaryAuthTokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authSvc.IsAuthEnabled() {
			if c.Query("token") != AuthDisabledBypassToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bypass token"})
				return
			}
			c.Set("user", &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"})
			c.Next()
			return
		}
		tok := c.Query("token")
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		user, err := tempTokenSvc.Validate(c.Request.Context(), tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}
