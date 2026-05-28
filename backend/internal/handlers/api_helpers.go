package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
)

// apiV1RequireMultiUser returns true if the request can proceed.
// Returns false (and writes the Node-parity 401) when MultiUserMode is off.
func apiV1RequireMultiUser(c *gin.Context, cfg *config.Config) bool {
	if !cfg.MultiUserMode {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Instance is not in Multi-User mode. Method denied",
		})
		return false
	}
	return true
}
