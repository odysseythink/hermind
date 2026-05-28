package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

func RegisterAgentRoutes(r *gin.RouterGroup, rt *agent.Runtime, authSvc *services.AuthService, tempTokenSvc *services.TemporaryAuthTokenService) {
	r.GET("/agent-invocation/:uuid",
		middleware.WSValidatedRequest(authSvc, tempTokenSvc),
		rt.HandleWS,
	)
}
