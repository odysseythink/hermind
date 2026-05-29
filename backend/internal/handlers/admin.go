package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type AdminHandler struct {
	adminSvc  *services.AdminService
	sysSvc    *services.SystemService
	wsSvc     *services.WorkspaceService
	apiKeySvc *services.APIKeyService
}

func NewAdminHandler(adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, apiKeySvc *services.APIKeyService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc, sysSvc: sysSvc, wsSvc: wsSvc, apiKeySvc: apiKeySvc}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.adminSvc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	target, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if target == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "User not found"})
		return
	}
	if ok, errStr := h.adminSvc.ValidCanModify(currUser, target); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if err := h.adminSvc.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *AdminHandler) ListWorkspaces(c *gin.Context) {
	workspaces, err := h.adminSvc.ListWorkspaces(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *AdminHandler) CreateInvite(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req dto.CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	invite, err := h.adminSvc.CreateInvite(c.Request.Context(), user.ID, req.WorkspaceIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite": invite, "error": nil})
}

func (h *AdminHandler) ListInvites(c *gin.Context) {
	invites, err := h.adminSvc.ListInvites(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invites": invites})
}

func (h *AdminHandler) DeactivateInvite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.adminSvc.DeactivateInvite(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) SystemPreferencesFor(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	labelsParam := c.Query("labels")
	labels := []string{}
	if labelsParam != "" {
		labels = strings.Split(labelsParam, ",")
	}

	publicFields := map[string]bool{
		"footer_data":                     true,
		"support_email":                   true,
		"text_splitter_chunk_size":        true,
		"text_splitter_chunk_overlap":     true,
		"max_embed_chunk_size":            true,
		"agent_search_provider":           true,
		"agent_sql_connections":           true,
		"default_agent_skills":            true,
		"disabled_agent_skills":           true,
		"disabled_filesystem_skills":      true,
		"disabled_create_files_skills":    true,
		"disabled_gmail_skills":           true,
		"gmail_agent_config":              true,
		"disabled_google_calendar_skills": true,
		"google_calendar_agent_config":    true,
		"disabled_outlook_skills":         true,
		"outlook_agent_config":            true,
		"imported_agent_skills":           true,
		"custom_app_name":                 true,
		"feature_flags":                   true,
		"meta_page_title":                 true,
		"meta_page_favicon":               true,
	}

	managerAllowedFields := map[string]bool{
		"custom_app_name":   true,
		"footer_data":       true,
		"support_email":     true,
		"meta_page_title":   true,
		"meta_page_favicon": true,
	}

	noRecord := map[string]bool{
		"max_embed_chunk_size":  true,
		"agent_sql_connections": true,
		"imported_agent_skills": true,
		"feature_flags":         true,
		"meta_page_title":       true,
		"meta_page_favicon":     true,
	}

	requestedSettings := make(map[string]any)
	for _, label := range labels {
		if !publicFields[label] {
			continue
		}
		if user.Role == "manager" && !managerAllowedFields[label] {
			continue
		}

		var setting string
		if !noRecord[label] {
			setting, _ = h.sysSvc.GetSetting(c.Request.Context(), label)
		}

		switch label {
		case "footer_data":
			if setting == "" {
				setting = "[]"
			}
			requestedSettings[label] = setting
		case "support_email", "agent_search_provider", "custom_app_name":
			if setting == "" {
				requestedSettings[label] = nil
			} else {
				requestedSettings[label] = setting
			}
		case "text_splitter_chunk_size":
			if setting == "" {
				requestedSettings[label] = nil
			} else {
				requestedSettings[label] = setting
			}
		case "text_splitter_chunk_overlap":
			if setting == "" {
				requestedSettings[label] = nil
			} else {
				requestedSettings[label] = setting
			}
		case "max_embed_chunk_size":
			requestedSettings[label] = 1000
		case "default_agent_skills", "disabled_agent_skills",
			"disabled_filesystem_skills", "disabled_create_files_skills",
			"disabled_gmail_skills", "disabled_google_calendar_skills",
			"disabled_outlook_skills":
			if setting == "" {
				requestedSettings[label] = []any{}
			} else {
				var arr []any
				if err := json.Unmarshal([]byte(setting), &arr); err != nil {
					requestedSettings[label] = []any{setting}
				} else {
					requestedSettings[label] = arr
				}
			}
		case "feature_flags":
			if setting == "" {
				requestedSettings[label] = map[string]any{}
			} else {
				var obj map[string]any
				if err := json.Unmarshal([]byte(setting), &obj); err != nil {
					requestedSettings[label] = map[string]any{}
				} else {
					requestedSettings[label] = obj
				}
			}
		case "agent_sql_connections":
			if setting == "" {
				requestedSettings[label] = []any{}
			} else {
				var arr []any
				if err := json.Unmarshal([]byte(setting), &arr); err != nil {
					requestedSettings[label] = []any{}
				} else {
					requestedSettings[label] = arr
				}
			}
		case "imported_agent_skills":
			requestedSettings[label] = []any{}
		default:
			if setting == "" {
				requestedSettings[label] = nil
			} else {
				requestedSettings[label] = setting
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"settings": requestedSettings})
}

func (h *AdminHandler) UpdateSystemPreferences(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	supportedFields := map[string]bool{
		"logo_filename":                   true,
		"telemetry_id":                    true,
		"footer_data":                     true,
		"support_email":                   true,
		"text_splitter_chunk_size":        true,
		"text_splitter_chunk_overlap":     true,
		"agent_search_provider":           true,
		"default_agent_skills":            true,
		"disabled_agent_skills":           true,
		"disabled_filesystem_skills":      true,
		"disabled_create_files_skills":    true,
		"disabled_gmail_skills":           true,
		"gmail_agent_config":              true,
		"disabled_google_calendar_skills": true,
		"google_calendar_agent_config":    true,
		"disabled_outlook_skills":         true,
		"outlook_agent_config":            true,
		"agent_sql_connections":           true,
		"custom_app_name":                 true,
		"default_system_prompt":           true,
		"meta_page_title":                 true,
		"meta_page_favicon":               true,
		"experimental_live_file_sync":     true,
		"hub_api_key":                     true,
	}

	managerAllowedFields := map[string]bool{
		"custom_app_name":   true,
		"footer_data":       true,
		"support_email":     true,
		"meta_page_title":   true,
		"meta_page_favicon": true,
	}

	// Filter unsupported fields
	for key := range updates {
		if !supportedFields[key] {
			delete(updates, key)
		}
	}

	// Manager role filter
	if user.Role == "manager" {
		for key := range updates {
			if !managerAllowedFields[key] {
				delete(updates, key)
			}
		}
	}

	for key, val := range updates {
		var strVal string
		switch key {
		case "footer_data":
			// Ensure it's a valid JSON array, max 3 items
			var arr []map[string]string
			switch v := val.(type) {
			case string:
				if err := json.Unmarshal([]byte(v), &arr); err != nil {
					arr = []map[string]string{}
				}
			case []any:
				b, _ := json.Marshal(v)
				_ = json.Unmarshal(b, &arr)
			default:
				arr = []map[string]string{}
			}
			if len(arr) > 3 {
				arr = arr[:3]
			}
			b, _ := json.Marshal(arr)
			strVal = string(b)
		case "default_agent_skills", "disabled_agent_skills",
			"disabled_filesystem_skills", "disabled_create_files_skills",
			"disabled_gmail_skills", "disabled_google_calendar_skills",
			"disabled_outlook_skills":
			// Frontend sends comma-separated strings (e.g. "a,b,c"); store as JSON array.
			switch v := val.(type) {
			case string:
				parts := strings.Split(v, ",")
				var arr []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						arr = append(arr, p)
					}
				}
				b, _ := json.Marshal(arr)
				strVal = string(b)
			case []any:
				b, _ := json.Marshal(v)
				strVal = string(b)
			default:
				b, _ := json.Marshal(v)
				strVal = string(b)
			}
		case "text_splitter_chunk_size", "text_splitter_chunk_overlap":
			switch v := val.(type) {
			case float64:
				strVal = fmt.Sprintf("%d", int(v))
			case string:
				strVal = v
			default:
				strVal = fmt.Sprintf("%v", val)
			}
		default:
			switch v := val.(type) {
			case string:
				strVal = v
			case float64:
				strVal = fmt.Sprintf("%v", v)
			case bool:
				strVal = fmt.Sprintf("%v", v)
			case nil:
				strVal = ""
			default:
				b, _ := json.Marshal(v)
				strVal = string(b)
			}
		}
		if err := h.sysSvc.SetSetting(c.Request.Context(), key, strVal); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *AdminHandler) ListWorkspaceUsers(c *gin.Context) {
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"users": []any{}})
		return
	}
	users, err := h.wsSvc.ListWorkspaceUsers(c.Request.Context(), wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"users": []any{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AdminHandler) ListAPIKeys(c *gin.Context) {
	keys, err := h.apiKeySvc.ListWithUser(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"apiKey": nil, "error": "Could not find an API Keys."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKeys": keys, "error": nil})
}

func (h *AdminHandler) GenerateAPIKey(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body struct {
		Name *string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	createdBy := &currUser.ID
	key, err := h.apiKeySvc.Create(c.Request.Context(), createdBy, body.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"apiKey": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKey": key, "error": nil})
}

func (h *AdminHandler) DeleteAPIKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.apiKeySvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.Status(http.StatusOK) // empty body, matches Node
}

func (h *AdminHandler) DeleteWorkspaceByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	// fetch slug first so we can best-effort delete vector namespace
	ws, err := h.wsSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if ws == nil {
		c.Status(http.StatusNotFound)
		return
	}
	found, err := h.wsSvc.DeleteByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if !found {
		c.Status(http.StatusNotFound)
		return
	}
	// best-effort: vector namespace cleanup. We don't have a vector svc
	// reference in AdminHandler in P4a — defer to P5. Comment marks the gap.
	_ = ws.Slug
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *AdminHandler) UpdateWorkspaceUsers(c *gin.Context) {
	wsID, err := strconv.Atoi(c.Param("workspaceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	var body struct {
		UserIDs []int `json:"userIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.wsSvc.UpdateUsers(c.Request.Context(), wsID, body.UserIDs); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *AdminHandler) CreateWorkspace(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"workspace": nil, "error": err.Error()})
		return
	}
	ws, err := h.wsSvc.Create(c.Request.Context(), currUser.ID, dto.CreateWorkspaceRequest{Name: body.Name})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "error": nil})
}

func (h *AdminHandler) CreateUserNew(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"user": nil, "error": err.Error()})
		return
	}
	if ok, errStr := h.adminSvc.ValidRoleSelection(currUser, body); !ok {
		c.JSON(http.StatusOK, gin.H{"user": nil, "error": errStr})
		return
	}
	in := services.CreateUserInput{
		Username: asString(body["username"]),
		Password: asString(body["password"]),
		Role:     asString(body["role"]),
		Bio:      asString(body["bio"]),
	}
	if dml, ok := body["dailyMessageLimit"].(float64); ok {
		v := int(dml)
		in.DailyMessageLimit = &v
	}
	u, businessErr, sysErr := h.adminSvc.CreateUser(c.Request.Context(), in)
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"user": nil, "error": sysErr.Error()})
		return
	}
	if businessErr != "" {
		c.JSON(http.StatusOK, gin.H{"user": nil, "error": businessErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "error": nil})
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	currUser := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	target, err := h.adminSvc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if target == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "User not found"})
		return
	}
	if ok, errStr := h.adminSvc.ValidCanModify(currUser, target); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if ok, errStr := h.adminSvc.ValidRoleSelection(currUser, updates); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if ok, errStr := h.adminSvc.CanModifyAdmin(target, updates); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errStr})
		return
	}
	if businessErr, sysErr := h.adminSvc.UpdateUser(c.Request.Context(), id, updates); sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
		return
	} else if businessErr != "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": businessErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func RegisterAdminRoutes(r *gin.RouterGroup, adminSvc *services.AdminService, sysSvc *services.SystemService, wsSvc *services.WorkspaceService, apiKeySvc *services.APIKeyService, authSvc *services.AuthService) {
	h := NewAdminHandler(adminSvc, sysSvc, wsSvc, apiKeySvc)
	r.GET("/admin/users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListUsers)
	r.POST("/admin/users/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.CreateUserNew)
	r.POST("/admin/user/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.UpdateUser)
	r.DELETE("/admin/user/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.DeleteUser)
	r.GET("/admin/workspaces/:workspaceId/users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.ListWorkspaceUsers)
	r.GET("/admin/api-keys",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListAPIKeys)
	r.POST("/admin/generate-api-key",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.GenerateAPIKey)
	r.DELETE("/admin/delete-api-key/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteAPIKey)
	r.DELETE("/admin/workspaces/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.DeleteWorkspaceByID)
	r.POST("/admin/workspaces/:workspaceId/update-users",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.UpdateWorkspaceUsers)
	r.POST("/admin/workspaces/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.CreateWorkspace)
	r.GET("/admin/workspaces",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListWorkspaces)
	r.POST("/admin/invite/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.CreateInvite)
	r.GET("/admin/invites",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListInvites)
	r.DELETE("/admin/invite/:id",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeactivateInvite)
	r.GET("/admin/system-preferences-for",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.SystemPreferencesFor)
	r.POST("/admin/system-preferences",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin", "manager"}),
		h.UpdateSystemPreferences)
}
