package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
)

type MCPHandler struct {
	svc      *services.MCPService
	eventLog *services.EventLogService
	cfg      *config.Config
}

func NewMCPHandler(svc *services.MCPService, eventLog *services.EventLogService, cfg *config.Config) *MCPHandler {
	return &MCPHandler{svc: svc, eventLog: eventLog, cfg: cfg}
}

func (h *MCPHandler) ListServers(c *gin.Context) {
	servers, err := h.svc.Servers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "servers": servers})
}

func (h *MCPHandler) ForceReload(c *gin.Context) {
	servers, err := h.svc.Reload(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "servers": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil, "servers": servers})
}

type mcpNameBody struct {
	Name string `json:"name"`
}

func (h *MCPHandler) ToggleServer(c *gin.Context) {
	var body mcpNameBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "name is required"})
		return
	}
	_, err := h.svc.ToggleServer(c.Request.Context(), body.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *MCPHandler) DeleteServer(c *gin.Context) {
	var body mcpNameBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "name is required"})
		return
	}
	ok, err := h.svc.DeleteServer(c.Request.Context(), body.Name)
	if err != nil {
		if errors.Is(err, mcp.ErrServerNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "MCP server " + body.Name + " not found in config file."})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "MCP server " + body.Name + " not found in config file."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

type mcpToggleToolBody struct {
	ServerName string `json:"serverName"`
	ToolName   string `json:"toolName"`
	Enabled    bool   `json:"enabled"`
}

func (h *MCPHandler) ToggleTool(c *gin.Context) {
	var body mcpToggleToolBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ServerName == "" || body.ToolName == "" {
		c.JSON(http.StatusOK, gin.H{
			"success":         false,
			"error":           "serverName and toolName are required",
			"suppressedTools": []string{},
		})
		return
	}
	suppressed, err := h.svc.ToggleTool(c.Request.Context(), body.ServerName, body.ToolName, body.Enabled)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":         false,
			"error":           err.Error(),
			"suppressedTools": []string{},
		})
		return
	}
	if suppressed == nil {
		suppressed = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"error":           nil,
		"suppressedTools": suppressed,
	})
}

const maxCallBodyBytes = 10 << 20 // 10 MiB

type mcpCallToolBody struct {
	Arguments map[string]any `json:"arguments"`
}

func (h *MCPHandler) CallTool(c *gin.Context) {
	server := c.Param("name")
	tool := c.Param("tool")
	if server == "" || tool == "" {
		respondCodedError(c, mcp.CodeInvalidParams, "server name and tool are required", nil)
		return
	}

	// 1. Body size cap
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCallBodyBytes)
	var body mcpCallToolBody
	if err := c.ShouldBindJSON(&body); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			respondCodedError(c, mcp.CodeBodyTooLarge, "request body exceeds 10 MiB cap", nil)
			return
		}
		respondCodedError(c, mcp.CodeInvalidBody, "invalid JSON body: "+err.Error(), nil)
		return
	}
	if body.Arguments == nil {
		body.Arguments = map[string]any{}
	}

	// 2. Per-call timeout
	timeout, err := parseTimeoutParam(c.Query("timeout"))
	if err != nil {
		respondCodedError(c, mcp.CodeInvalidParams, err.Error(), nil)
		return
	}
	if timeout == 0 {
		timeout = h.cfg.MCPCallTimeoutDefault
		if timeout == 0 {
			timeout = 30 * time.Second
		}
	}

	// 3. Schema lookup & validation
	toolSchema, err := h.svc.GetToolSchema(server, tool)
	if err != nil {
		switch {
		case errors.Is(err, mcp.ErrServerNotFound):
			respondCodedError(c, mcp.CodeServerNotFound, err.Error(), nil)
		case errors.Is(err, mcp.ErrToolNotFound):
			respondCodedError(c, mcp.CodeToolNotFound, err.Error(), nil)
		default:
			respondCodedError(c, mcp.CodeInternalError, err.Error(), nil)
		}
		return
	}
	if err := validateArgsAgainstSchema(body.Arguments, toolSchema.InputSchema); err != nil {
		respondCodedError(c, mcp.CodeArgsSchemaMismatch, err.Error(), gin.H{
			"schema_url": fmt.Sprintf("inputSchema of tool %s/%s", server, tool),
		})
		return
	}

	// 4. Concurrency gate
	if !h.svc.TryAcquireCall(server) {
		h.logAuditAsync(c, "mcp.call.rejected", server, tool, 0, mcp.CodeConcurrencyLimit, nil)
		respondCodedError(c, mcp.CodeConcurrencyLimit, "per-server in-flight cap reached, retry later", nil)
		return
	}
	defer h.svc.ReleaseCall(server)

	// 5. Dispatch with deadline
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	start := time.Now()
	result, err := h.svc.CallTool(ctx, server, tool, body.Arguments)
	dur := time.Since(start)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			h.logAuditAsync(c, "mcp.call.failed", server, tool, dur, mcp.CodeCallTimeout, nil)
			respondCodedError(c, mcp.CodeCallTimeout, fmt.Sprintf("call exceeded %s", timeout), nil)
			return
		}
		h.logAuditAsync(c, "mcp.call.failed", server, tool, dur, mcp.CodeTransportError, gin.H{
			"transport_error": err.Error(),
		})
		respondCodedError(c, mcp.CodeTransportError, err.Error(), nil)
		return
	}

	h.logAuditAsync(c, "mcp.call.success", server, tool, dur, "", nil)
	respondCallSuccess(c, result)
}

func (h *MCPHandler) logAuditAsync(c *gin.Context, event, server, tool string, dur time.Duration, code mcp.ErrorCode, extra gin.H) {
	if h.eventLog == nil {
		return
	}
	userVal, _ := c.Get("user")
	var userID *int
	if u, ok := userVal.(*models.User); ok && u != nil {
		userID = &u.ID
	}
	meta := gin.H{
		"server":      server,
		"tool":        tool,
		"duration_ms": dur.Milliseconds(),
	}
	if code != "" {
		meta["error_code"] = string(code)
	}
	for k, v := range extra {
		meta[k] = v
	}
	go func() {
		// Detach from request ctx so a client disconnect doesn't drop the log.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.eventLog.LogEvent(ctx, event, meta, userID); err != nil {
			mlog.Warning("mcp audit log failed", mlog.String("event", event), mlog.Err(err))
		}
	}()
}

// RegisterMCPRoutes wires the 5 Node-parity admin routes + 1 Go-private
// tool-call proxy under the supplied router group. All routes require an
// authenticated admin user.
func RegisterMCPRoutes(r *gin.RouterGroup, authSvc *services.AuthService, svc *services.MCPService, eventLog *services.EventLogService, cfg *config.Config) {
	h := NewMCPHandler(svc, eventLog, cfg)
	admin := []gin.HandlerFunc{
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
	}

	r.GET("/mcp-servers/force-reload", append(admin, h.ForceReload)...)
	r.GET("/mcp-servers/list", append(admin, h.ListServers)...)
	r.POST("/mcp-servers/toggle", append(admin, h.ToggleServer)...)
	r.POST("/mcp-servers/delete", append(admin, h.DeleteServer)...)
	r.POST("/mcp-servers/toggle-tool", append(admin, h.ToggleTool)...)

	// Go-private tool-call proxy (not Node-compat)
	r.POST("/mcp/:name/tools/:tool/call", append(admin, h.CallTool)...)
}
