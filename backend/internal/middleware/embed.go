package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

func ValidEmbedConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		embedUUID := c.Param("embedId")
		var embed models.EmbedConfig
		if err := db.Where("uuid = ?", embedUUID).Preload("Workspace").First(&embed).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed config not found"})
			c.Abort()
			return
		}
		c.Set("embedConfig", &embed)
		c.Next()
	}
}

func ValidEmbedConfigId(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("embedId")
		var id int
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid embed ID"})
			c.Abort()
			return
		}
		var embed models.EmbedConfig
		if err := db.First(&embed, id).Error; err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "Embed config not found"})
			c.Abort()
			return
		}
		c.Set("embedConfig", &embed)
		c.Next()
	}
}

func SetConnectionMeta() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.GetHeader("Origin")
		if host == "" {
			host = c.GetHeader("Referer")
		}
		c.Set("connection", &dto.ConnectionMeta{Host: host, IP: c.ClientIP()})
		c.Next()
	}
}

func CanRespond(db *gorm.DB, embedSvc *services.EmbedService) gin.HandlerFunc {
	return func(c *gin.Context) {
		embed := c.MustGet("embedConfig").(*models.EmbedConfig)
		conn := c.MustGet("connection").(*dto.ConnectionMeta)

		var req dto.EmbedStreamChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeSSEAbort(c, "Invalid request body")
			c.Abort()
			return
		}

		// 1. Enabled check
		if !embed.Enabled {
			writeSSEAbort(c, "Embed is not enabled")
			c.Abort()
			return
		}

		// 2. Domain allowlist
		if embed.AllowlistDomains != nil {
			allowed, err := parseAllowlistDomains(*embed.AllowlistDomains)
			if err != nil || len(allowed) == 0 {
				writeSSEAbort(c, "Domain not allowed")
				c.Abort()
				return
			}
			if !isOriginAllowed(conn.Host, allowed) {
				writeSSEAbort(c, "Domain not allowed")
				c.Abort()
				return
			}
		}

		// 3. Session ID validation (UUID v4)
		if _, err := uuid.Parse(req.SessionID); err != nil {
			writeSSEAbort(c, "Invalid session ID")
			c.Abort()
			return
		}

		// 4. Message validation
		if strings.TrimSpace(req.Message) == "" {
			writeSSEAbort(c, "Message is required")
			c.Abort()
			return
		}
		if embed.ChatMode != "chat" && embed.ChatMode != "query" {
			writeSSEAbort(c, "Invalid chat mode")
			c.Abort()
			return
		}

		// 5. Daily rate limit (24h rolling window)
		since := time.Now().Add(-24 * time.Hour)
		if embed.MaxChatsPerDay != nil && *embed.MaxChatsPerDay > 0 {
			count := embedSvc.CountRecentChats(c.Request.Context(), embed.ID, since)
			if count >= int64(*embed.MaxChatsPerDay) {
				writeSSEAbort(c, "Daily chat limit exceeded")
				c.Abort()
				return
			}
		}

		// 6. Per-session rate limit (24h rolling window)
		if embed.MaxChatsPerSession != nil && *embed.MaxChatsPerSession > 0 {
			count := embedSvc.CountRecentSessionChats(c.Request.Context(), embed.ID, req.SessionID, since)
			if count >= int64(*embed.MaxChatsPerSession) {
				writeSSEAbort(c, "Session chat limit exceeded")
				c.Abort()
				return
			}
		}

		c.Set("embedRequest", &req)
		c.Next()
	}
}

func writeSSEAbort(c *gin.Context, msg string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	id := uuid.New().String()
	chunk := dto.StreamChatResponse{
		UUID:         id,
		Type:         "abort",
		TextResponse: nil,
		Sources:      []any{},
		Close:        true,
		Error:        &msg,
	}
	data, _ := json.Marshal(chunk)
	c.Writer.Write([]byte("data: "))
	c.Writer.Write(data)
	c.Writer.Write([]byte("\n\n"))
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func parseAllowlistDomains(s string) ([]string, error) {
	var domains []string
	if err := json.Unmarshal([]byte(s), &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

func isOriginAllowed(origin string, allowed []string) bool {
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := originURL.Host
	if originHost == "" {
		originHost = originURL.Path
	}
	for _, a := range allowed {
		allowedURL, err := url.Parse(a)
		if err != nil {
			continue
		}
		allowedHost := allowedURL.Host
		if allowedHost == "" {
			allowedHost = allowedURL.Path
		}
		if originHost == allowedHost {
			return true
		}
		if strings.HasSuffix(originHost, "."+allowedHost) {
			return true
		}
	}
	return false
}
