package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

func ValidatedRequest(authSvc *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Single-user mode without password: bypass auth (same as Node.js server)
		if !authSvc.IsAuthEnabled() {
			c.Set("user", &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"})
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "No auth token found"})
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		user, err := authSvc.ValidateToken(c.Request.Context(), tokenStr)
		if err != nil {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid auth token"})
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}
