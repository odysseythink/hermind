package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/mcp"
)

func codeToStatus(c mcp.ErrorCode) int {
	switch c {
	case mcp.CodeInvalidBody:
		return http.StatusBadRequest
	case mcp.CodeInvalidParams:
		return http.StatusUnprocessableEntity
	case mcp.CodeServerNotFound:
		return http.StatusNotFound
	case mcp.CodeToolNotFound:
		return http.StatusNotFound
	case mcp.CodeArgsSchemaMismatch:
		return http.StatusUnprocessableEntity
	case mcp.CodeBodyTooLarge:
		return http.StatusRequestEntityTooLarge
	case mcp.CodeConcurrencyLimit:
		return http.StatusTooManyRequests
	case mcp.CodeCallTimeout:
		return http.StatusGatewayTimeout
	case mcp.CodeTransportError:
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

func respondCodedError(c *gin.Context, code mcp.ErrorCode, msg string, details map[string]any) {
	body := gin.H{
		"success":   false,
		"result":    nil,
		"error":     msg,
		"errorCode": string(code),
	}
	if len(details) > 0 {
		body["details"] = details
	}
	c.JSON(codeToStatus(code), body)
}

func respondCallSuccess(c *gin.Context, result any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  result,
		"error":   nil,
	})
}
