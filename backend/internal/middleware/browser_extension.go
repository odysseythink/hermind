package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

func ValidBrowserExtensionApiKey(
	extSvc *services.BrowserExtensionService,
	authSvc *services.AuthService,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "No auth token found"})
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		apiKey, err := extSvc.Validate(c.Request.Context(), tokenStr)
		if err != nil {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
			c.Abort()
			return
		}
		if cfg.MultiUserMode {
			if apiKey.UserID == nil {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
				c.Abort()
				return
			}
			user, err := authSvc.GetUserByID(c.Request.Context(), *apiKey.UserID)
			if err != nil {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid browser extension API key"})
				c.Abort()
				return
			}
			if user.Suspended != 0 {
				c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "User account suspended"})
				c.Abort()
				return
			}
			c.Set("user", user)
		} else {
			c.Set("user", &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"})
		}
		c.Set("apiKey", apiKey)
		c.Next()
	}
}
