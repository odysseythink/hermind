package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
)

func IsMultiUserSetup(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.MultiUserMode {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid request"})
			return
		}
		c.Next()
	}
}
