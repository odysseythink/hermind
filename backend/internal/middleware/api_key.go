package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/services"
)

func ValidAPIKey(apiKeySvc *services.APIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "API key required"})
			c.Abort()
			return
		}
		key, err := apiKeySvc.ValidateKey(c.Request.Context(), tokenStr)
		if err != nil {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid API key"})
			c.Abort()
			return
		}
		c.Set("apiKey", key)
		c.Next()
	}
}
