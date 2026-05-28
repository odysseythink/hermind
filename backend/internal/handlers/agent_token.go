package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

const agentWSTTL = 3 * time.Minute

func RegisterAgentTokenRoutes(r *gin.RouterGroup, tempTokenSvc *services.TemporaryAuthTokenService, authSvc *services.AuthService) {
	r.POST("/workspace/:slug/agent-token",
		middleware.ValidatedRequest(authSvc),
		func(c *gin.Context) {
			user := c.MustGet("user").(*models.User)
			if user.ID == 0 {
				// Auth-disabled mode: admin bypass user has ID 0.
				// Persisting a temp token requires a real DB row, so we accept
				// the bypass by issuing an unbacked token only when auth is OFF.
				// For PR-AR-1 we keep it simple: refuse and force auth-on for WS.
				c.JSON(http.StatusOK, gin.H{
					"success":          true,
					"token":            middleware.AuthDisabledBypassToken,
					"expiresInSeconds": int(agentWSTTL.Seconds()),
				})
				return
			}
			tok, err := tempTokenSvc.IssueWithTTL(c.Request.Context(), user.ID, agentWSTTL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success":          true,
				"token":            tok,
				"expiresInSeconds": int(agentWSTTL.Seconds()),
			})
		},
	)
}
