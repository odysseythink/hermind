package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

func ValidWorkspaceAndThreadSlug(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		if slug == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Workspace slug required"})
			c.Abort()
			return
		}
		var ws models.Workspace
		if err := db.Where("slug = ?", slug).First(&ws).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Workspace not found"})
			c.Abort()
			return
		}
		c.Set("workspace", &ws)

		threadSlug := c.Param("threadSlug")
		if threadSlug == "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Thread slug required"})
			c.Abort()
			return
		}
		var thread models.WorkspaceThread
		if err := db.Where("slug = ? AND workspace_id = ?", threadSlug, ws.ID).First(&thread).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Thread not found"})
			c.Abort()
			return
		}
		c.Set("thread", &thread)
		c.Next()
	}
}
