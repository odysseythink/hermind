package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
)

func FlexUserRoleValid(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "No user in context"})
			c.Abort()
			return
		}
		user := userVal.(*models.User)
		if !slices.Contains(allowed, user.Role) && !slices.Contains(allowed, "all") {
			c.JSON(http.StatusForbidden, dto.ErrorResponse{Error: "Invalid permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}
