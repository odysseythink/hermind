package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

func RegisterAPIAuthRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService) {
	r.GET("/v1/auth",
		middleware.ValidAPIKey(apiKeySvc),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
		})
}
