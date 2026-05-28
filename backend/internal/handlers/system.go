package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type SystemHandler struct {
	sysSvc            *services.SystemService
	apiKeySvc         *services.APIKeyService
	adminSvc          *services.AdminService
	authSvc           *services.AuthService
	cfg               *config.Config
	fsSvc             *services.FileSystemService
	coll              *collector.Client
	vectorSvc         *services.VectorService
	promptPresetSvc   *services.PromptPresetService
	promptVariableSvc *services.PromptVariableService
	wsChatSvc         *services.WorkspaceChatService
}

func NewSystemHandler(sysSvc *services.SystemService, apiKeySvc *services.APIKeyService, adminSvc *services.AdminService, authSvc *services.AuthService, cfg *config.Config, fsSvc *services.FileSystemService, coll *collector.Client, vectorSvc *services.VectorService, promptPresetSvc *services.PromptPresetService, promptVariableSvc *services.PromptVariableService, wsChatSvc *services.WorkspaceChatService) *SystemHandler {
	return &SystemHandler{sysSvc: sysSvc, apiKeySvc: apiKeySvc, adminSvc: adminSvc, authSvc: authSvc, cfg: cfg, fsSvc: fsSvc, coll: coll, vectorSvc: vectorSvc, promptPresetSvc: promptPresetSvc, promptVariableSvc: promptVariableSvc, wsChatSvc: wsChatSvc}
}

func (h *SystemHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, dto.PingResponse{Status: "ok"})
}

func (h *SystemHandler) SetupComplete(c *gin.Context) {
	complete := h.sysSvc.IsSetupComplete(c.Request.Context())

	// Start with all settings from DB (written by update-env)
	dbSettings, _ := h.sysSvc.GetAllSettings(c.Request.Context())

	// Build a mixed-type results map (same as Node.js server behavior)
	results := gin.H{}
	for k, v := range dbSettings {
		results[k] = v
	}

	// Overlay critical fields with correct JSON types
	results["RequiresAuth"] = h.cfg.AuthToken != ""
	results["AuthToken"] = h.cfg.AuthToken != ""
	results["JWTSecret"] = h.cfg.JWTSecret != ""
	results["StorageDir"] = h.cfg.StorageDir
	results["MultiUserMode"] = h.cfg.MultiUserMode
	results["DisableTelemetry"] = false
	results["setupComplete"] = complete
	// Use DB settings first, cfg as fallback
	if _, ok := results["LLMProvider"]; !ok {
		results["LLMProvider"] = h.cfg.LLMProvider
	}
	if _, ok := results["VectorDB"]; !ok {
		results["VectorDB"] = h.cfg.VectorDB
	}
	if _, ok := results["EmbeddingEngine"]; !ok {
		results["EmbeddingEngine"] = h.cfg.EmbeddingEngine
	}

	// Derive LLMModel from the active provider's model preference (like Node.js server does)
	llmModel := h.cfg.LLMModel
	provider, _ := results["LLMProvider"].(string)
	switch provider {
	case "ollama":
		if v, ok := results["OllamaLLMModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "openai":
		if v, ok := results["OpenAiModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "anthropic":
		if v, ok := results["AnthropicModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "gemini":
		if v, ok := results["GeminiLLMModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "lmstudio":
		if v, ok := results["LMStudioModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "localai":
		if v, ok := results["LocalAiModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "groq":
		if v, ok := results["GroqModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "togetherai":
		if v, ok := results["TogetherAiModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "mistral":
		if v, ok := results["MistralModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "perplexity":
		if v, ok := results["PerplexityModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	case "deepseek":
		if v, ok := results["DeepSeekModelPref"].(string); ok && v != "" {
			llmModel = v
		}
	}
	results["LLMModel"] = llmModel

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *SystemHandler) GetSystemSettings(c *gin.Context) {
	settings, err := h.sysSvc.GetAllSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.SystemSettingsResponse{Settings: settings})
}

func (h *SystemHandler) UpdateSystemSetting(c *gin.Context) {
	var req dto.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.sysSvc.SetSetting(c.Request.Context(), req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SystemHandler) UpdateEnv(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"newValues": nil, "error": err.Error()})
		return
	}

	newValues := make(map[string]any)
	for key, val := range body {
		strVal := ""
		switch v := val.(type) {
		case string:
			strVal = v
		case float64:
			strVal = fmt.Sprintf("%v", v)
		case bool:
			strVal = fmt.Sprintf("%v", v)
		default:
			strVal = fmt.Sprintf("%v", v)
		}
		// Skip masked values (all asterisks) like Node.js server does
		if strings.Trim(strVal, "*") == "" && len(strVal) > 0 {
			continue
		}
		if err := h.sysSvc.SetSetting(c.Request.Context(), key, strVal); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"newValues": nil, "error": err.Error()})
			return
		}
		newValues[key] = strVal
	}

	c.JSON(http.StatusOK, gin.H{"newValues": newValues, "error": nil})
}

func (h *SystemHandler) GetOnboardingStatus(c *gin.Context) {
	status, _ := h.sysSvc.GetOnboardingStatus(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"onboardingComplete": status})
}

func (h *SystemHandler) GetSupportEmail(c *gin.Context) {
	email, _ := h.sysSvc.GetSetting(c.Request.Context(), "support_email")
	c.JSON(http.StatusOK, gin.H{"supportEmail": email})
}

func (h *SystemHandler) GetFooterData(c *gin.Context) {
	data, err := h.sysSvc.GetSetting(c.Request.Context(), "footer_data")
	if err != nil || data == "" {
		data = "[]"
	}
	c.JSON(http.StatusOK, gin.H{"footerData": data})
}

func (h *SystemHandler) DefaultSystemPrompt(c *gin.Context) {
	prompt, _ := h.sysSvc.GetSetting(c.Request.Context(), "default_system_prompt")
	if prompt == "" {
		prompt = "Given the following conversation, relevant context, and a follow up question, reply with an answer to the current question the user is asking. Return only your response to the question given the above information following the users instructions as needed."
	}
	c.JSON(http.StatusOK, gin.H{
		"success":                 true,
		"defaultSystemPrompt":     prompt,
		"saneDefaultSystemPrompt": "Given the following conversation, relevant context, and a follow up question, reply with an answer to the current question the user is asking. Return only your response to the question given the above information following the users instructions as needed.",
	})
}

func (h *SystemHandler) PromptVariables(c *gin.Context) {
	vars, err := h.promptVariableSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"variables": vars})
}

func (h *SystemHandler) IsDefaultLogo(c *gin.Context) {
	const defaultLogo = "hermind.png"
	currentLogo, _ := h.sysSvc.GetSetting(c.Request.Context(), "logo_filename")
	isDefault := currentLogo == "" || currentLogo == defaultLogo
	c.JSON(http.StatusOK, gin.H{"isDefaultLogo": isDefault})
}

func (h *SystemHandler) MultiUserMode(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"multiUserMode": h.cfg.MultiUserMode})
}

func (h *SystemHandler) ApiKeys(c *gin.Context) {
	if h.cfg.MultiUserMode {
		c.Status(http.StatusUnauthorized)
		return
	}
	keys, err := h.apiKeySvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"apiKey": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKeys": keys, "error": nil})
}

func (h *SystemHandler) GenerateApiKey(c *gin.Context) {
	if h.cfg.MultiUserMode {
		c.Status(http.StatusUnauthorized)
		return
	}
	var req struct {
		Name *string `json:"name"`
	}
	c.ShouldBindJSON(&req)
	user := c.MustGet("user").(*models.User)
	key, err := h.apiKeySvc.Create(c.Request.Context(), &user.ID, req.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"apiKey": nil, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKey": key, "error": nil})
}

func (h *SystemHandler) DeleteApiKey(c *gin.Context) {
	if h.cfg.MultiUserMode {
		c.Status(http.StatusUnauthorized)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	if err := h.apiKeySvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SystemHandler) EventLogs(c *gin.Context) {
	var req struct {
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Offset = 0
		req.Limit = 10
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	c.JSON(http.StatusOK, gin.H{
		"logs":      []any{},
		"hasPages":  false,
		"totalLogs": 0,
	})
}

func (h *SystemHandler) ClearEventLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SystemHandler) Metrics(c *gin.Context) {
	mode := "single-user"
	if h.cfg.MultiUserMode {
		mode = "multi-user"
	}
	c.JSON(http.StatusOK, gin.H{
		"online":     true,
		"version":    "--",
		"mode":       mode,
		"vectorDB":   h.cfg.VectorDB,
		"storage":    gin.H{},
		"appVersion": "1.0.0",
	})
}

func (h *SystemHandler) CompleteOnboarding(c *gin.Context) {
	if err := h.sysSvc.CompleteOnboarding(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *SystemHandler) CustomModels(c *gin.Context) {
	var req dto.CustomModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"models": []any{}, "error": err.Error()})
		return
	}

	switch req.Provider {
	case "ollama":
		models, err := h.ollamaModels(req.BasePath, req.APIKey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"models": []any{}, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models, "error": nil})
	default:
		c.JSON(http.StatusOK, gin.H{"models": []any{}, "error": nil})
	}
}

func (h *SystemHandler) ollamaModels(basePath *string, authToken *string) ([]gin.H, error) {
	urlStr := ""
	if basePath != nil && *basePath != "" {
		urlStr = *basePath
	}
	if urlStr == "" {
		return nil, fmt.Errorf("no base path provided")
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("not a valid URL")
	}
	if strings.HasSuffix(u.Path, "/") {
		return nil, fmt.Errorf("base path cannot end in /")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/tags", urlStr), nil)
	if err != nil {
		return nil, err
	}
	if authToken != nil && *authToken != "" {
		req.Header.Set("Authorization", "Bearer "+*authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not reach ollama server: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	models := make([]gin.H, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, gin.H{"id": m.Name})
	}
	return models, nil
}

func (h *SystemHandler) CheckToken(c *gin.Context) {
	if !h.cfg.MultiUserMode {
		c.Status(http.StatusOK)
		return
	}
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil || user.Suspended != 0 {
		c.Status(http.StatusForbidden)
		return
	}
	c.Status(http.StatusOK)
}

func (h *SystemHandler) UpdateOwnUser(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil || user.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}
	var req struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
		Bio      *string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	bizErr, sysErr := h.authSvc.UpdateOwnProfile(c.Request.Context(), user, req.Username, req.Password, req.Bio)
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
		return
	}
	if bizErr != "" {
		// Match Node: "No updates provided" → 400; other business errors → 200 with error.
		if bizErr == "No updates provided" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": bizErr})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *SystemHandler) EnableMultiUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	user, bizErr, sysErr := h.authSvc.EnableMultiUser(c.Request.Context(), h.adminSvc, h.sysSvc, req.Username, req.Password)
	if sysErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": sysErr.Error()})
		return
	}
	if bizErr != "" {
		// Node returns 200 for "already enabled" branch but 400 for create-failed branch.
		// Distinguish by checking the cfg state: if MultiUserMode already on, send 200.
		if bizErr == "Multi-user mode is already enabled." {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": bizErr})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": bizErr})
		return
	}
	_ = user // future: include user.id in response if needed
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *SystemHandler) UpdatePassword(c *gin.Context) {
	if h.cfg.MultiUserMode {
		c.Status(http.StatusUnauthorized)
		return
	}
	var req dto.UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.authSvc.RotateCredentials(c.Request.Context(), h.sysSvc, req.UsePassword, req.NewPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

func (h *SystemHandler) RefreshUser(c *gin.Context) {
	if !h.cfg.MultiUserMode {
		c.JSON(http.StatusOK, gin.H{"success": true, "user": nil, "message": nil})
		return
	}
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "user": nil, "message": "Session expired or invalid."})
		return
	}
	if user.Suspended != 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "user": nil, "message": "User is suspended."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "user": services.FilterUserFields(user), "message": nil})
}

// Migrate returns 200 — used by frontend as a startup probe.
func (h *SystemHandler) Migrate(c *gin.Context) {
	c.Status(http.StatusOK)
}

// EnvDump returns 200 (noop in Go server; Node dumps env in production).
func (h *SystemHandler) EnvDump(c *gin.Context) {
	c.Status(http.StatusOK)
}

// LocalFiles returns the list of files/folders in the documents directory.
func (h *SystemHandler) LocalFiles(c *gin.Context) {
	files, err := h.fsSvc.ListLocalFiles("")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"localFiles": []any{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"localFiles": files})
}

// AcceptedDocumentTypes returns the collector-supported file types.
func (h *SystemHandler) AcceptedDocumentTypes(c *gin.Context) {
	if h.coll == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	types, err := h.coll.AcceptedFileTypes(c.Request.Context())
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{"types": types})
}

// RemoveDocument removes a single document from filesystem.
func (h *SystemHandler) RemoveDocument(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.fsSvc.RemoveDocument(req.Name); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// RemoveDocuments removes multiple documents from filesystem.
func (h *SystemHandler) RemoveDocuments(c *gin.Context) {
	var req struct {
		Names []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	for _, name := range req.Names {
		if err := h.fsSvc.RemoveDocument(name); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
	}
	c.Status(http.StatusOK)
}

// RemoveFolder removes a folder and its contents from filesystem.
func (h *SystemHandler) RemoveFolder(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.fsSvc.RemoveFolder(req.Name); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

// CustomAppName returns the custom app name setting.
func (h *SystemHandler) CustomAppName(c *gin.Context) {
	name, _ := h.sysSvc.GetSetting(c.Request.Context(), "custom_app_name")
	c.JSON(http.StatusOK, gin.H{"customAppName": name})
}

// UpdateDefaultSystemPrompt updates the default system prompt setting.
func (h *SystemHandler) UpdateDefaultSystemPrompt(c *gin.Context) {
	var req struct {
		DefaultSystemPrompt string `json:"defaultSystemPrompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := h.sysSvc.SetSetting(c.Request.Context(), "default_system_prompt", req.DefaultSystemPrompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Default system prompt updated successfully."})
}

// SystemVectors returns the total vector count (or per-namespace if slug query provided).
func (h *SystemHandler) SystemVectors(c *gin.Context) {
	slug := c.Query("slug")
	var count int64
	var err error
	if slug != "" {
		if h.vectorSvc == nil {
			c.JSON(http.StatusOK, gin.H{"vectorCount": 0})
			return
		}
		count, err = h.vectorSvc.CountVectors(c.Request.Context(), slug)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"vectorCount": 0})
			return
		}
	} else {
		if h.vectorSvc == nil {
			c.JSON(http.StatusOK, gin.H{"vectorCount": 0})
			return
		}
		count, err = h.vectorSvc.TotalVectors(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"vectorCount": 0})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"vectorCount": count})
}

// DocumentProcessingStatus returns 200 if collector is online, 503 otherwise.
func (h *SystemHandler) DocumentProcessingStatus(c *gin.Context) {
	if h.coll == nil || !h.coll.Online(c.Request.Context()) {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.Status(http.StatusOK)
}

// GetPfp serves a user's profile picture.
func (h *SystemHandler) GetPfp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil || user.ID != id {
		c.Status(http.StatusNoContent)
		return
	}

	pfpDir := h.fsSvc.PfpDir()
	pfpFilename := ""
	if user.PfpFilename != nil {
		pfpFilename = *user.PfpFilename
	}
	if pfpFilename == "" {
		c.Status(http.StatusNoContent)
		return
	}

	pfpPath := filepath.Join(pfpDir, pfpFilename)
	if !h.fsSvc.IsWithin(pfpDir, pfpPath) {
		c.Status(http.StatusNoContent)
		return
	}

	found, data, size, mimeType, err := h.fsSvc.ReadAsset(pfpPath)
	if err != nil || !found {
		c.Status(http.StatusNoContent)
		return
	}

	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(pfpPath)))
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.Data(http.StatusOK, mimeType, data)
}

// UploadPfp handles profile picture upload.
func (h *SystemHandler) UploadPfp(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user session."})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "File upload failed."})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	newFilename := uuid.New().String() + ext
	_, err = h.fsSvc.SavePfp(newFilename, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save profile picture."})
		return
	}

	// Remove old PFP if exists
	if user.PfpFilename != nil && *user.PfpFilename != "" {
		oldPath := filepath.Join(h.fsSvc.PfpDir(), *user.PfpFilename)
		if h.fsSvc.IsWithin(h.fsSvc.PfpDir(), oldPath) {
			_ = h.fsSvc.RemoveAsset(oldPath)
		}
	}

	if err := h.authSvc.UpdatePfp(c.Request.Context(), user.ID, &newFilename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update profile picture."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile picture uploaded successfully."})
}

// RemovePfp removes a user's profile picture.
func (h *SystemHandler) RemovePfp(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, _ := userVal.(*models.User)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid user session."})
		return
	}

	if user.PfpFilename != nil && *user.PfpFilename != "" {
		oldPath := filepath.Join(h.fsSvc.PfpDir(), *user.PfpFilename)
		if h.fsSvc.IsWithin(h.fsSvc.PfpDir(), oldPath) {
			_ = h.fsSvc.RemoveAsset(oldPath)
		}
	}

	if err := h.authSvc.UpdatePfp(c.Request.Context(), user.ID, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to remove profile picture."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile picture removed successfully."})
}

// Logo serves the current logo (custom or default).
func (h *SystemHandler) Logo(c *gin.Context) {
	const defaultLogo = "hermind.png"
	const defaultLogoDark = "hermind-dark.png"

	darkMode := c.Query("theme") == "" || c.Query("theme") == "default"
	defaultFilename := defaultLogoDark
	if darkMode {
		defaultFilename = defaultLogo
	}

	currentLogo, _ := h.sysSvc.GetSetting(c.Request.Context(), "logo_filename")
	assetsDir := h.fsSvc.AssetsDir()

	var logoPath string
	if currentLogo != "" && currentLogo != defaultLogo && currentLogo != defaultLogoDark {
		customPath := filepath.Join(assetsDir, currentLogo)
		if h.fsSvc.IsWithin(assetsDir, customPath) {
			if _, err := os.Stat(customPath); err == nil {
				logoPath = customPath
			}
		}
	}
	if logoPath == "" {
		logoPath = filepath.Join(assetsDir, defaultFilename)
	}

	found, data, size, mimeType, err := h.fsSvc.ReadAsset(logoPath)
	if err != nil || !found {
		c.Status(http.StatusNoContent)
		return
	}

	isCustom := currentLogo != "" && currentLogo != defaultLogo && currentLogo != defaultLogoDark
	c.Header("Access-Control-Expose-Headers", "Content-Disposition,X-Is-Custom-Logo,Content-Type,Content-Length")
	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(logoPath)))
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.Header("X-Is-Custom-Logo", fmt.Sprintf("%v", isCustom))
	c.Data(http.StatusOK, mimeType, data)
}

// UploadLogo handles logo upload.
func (h *SystemHandler) UploadLogo(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "No logo file provided."})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	newFilename := uuid.New().String() + ext
	_, err = h.fsSvc.SaveAsset(newFilename, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save logo."})
		return
	}

	// Remove old custom logo if exists
	currentLogo, _ := h.sysSvc.GetSetting(c.Request.Context(), "logo_filename")
	if currentLogo != "" && currentLogo != "hermind.png" && currentLogo != "hermind-dark.png" {
		oldPath := filepath.Join(h.fsSvc.AssetsDir(), currentLogo)
		if h.fsSvc.IsWithin(h.fsSvc.AssetsDir(), oldPath) {
			_ = h.fsSvc.RemoveAsset(oldPath)
		}
	}

	if err := h.sysSvc.SetSetting(c.Request.Context(), "logo_filename", newFilename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update logo setting."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logo uploaded successfully."})
}

// RemoveLogo removes the custom logo and resets to default.
func (h *SystemHandler) RemoveLogo(c *gin.Context) {
	currentLogo, _ := h.sysSvc.GetSetting(c.Request.Context(), "logo_filename")
	if currentLogo != "" && currentLogo != "hermind.png" && currentLogo != "hermind-dark.png" {
		oldPath := filepath.Join(h.fsSvc.AssetsDir(), currentLogo)
		if h.fsSvc.IsWithin(h.fsSvc.AssetsDir(), oldPath) {
			_ = h.fsSvc.RemoveAsset(oldPath)
		}
	}

	if err := h.sysSvc.SetSetting(c.Request.Context(), "logo_filename", "hermind.png"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to remove logo."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logo removed successfully."})
}

func RegisterSystemRoutes(r *gin.RouterGroup, sysSvc *services.SystemService, apiKeySvc *services.APIKeyService, cfg *config.Config, authSvc *services.AuthService, adminSvc *services.AdminService, fsSvc *services.FileSystemService, coll *collector.Client, vectorSvc *services.VectorService, promptPresetSvc *services.PromptPresetService, promptVariableSvc *services.PromptVariableService, wsChatSvc *services.WorkspaceChatService) {
	h := NewSystemHandler(sysSvc, apiKeySvc, adminSvc, authSvc, cfg, fsSvc, coll, vectorSvc, promptPresetSvc, promptVariableSvc, wsChatSvc)
	r.GET("/ping", h.Ping)
	r.GET("/setup-complete", h.SetupComplete)
	r.GET("/migrate", h.Migrate)
	r.GET("/env-dump", h.EnvDump)
	r.GET("/system", h.GetSystemSettings)
	r.POST("/system", h.UpdateSystemSetting)
	r.POST("/system/update-env", h.UpdateEnv)
	r.POST("/system/custom-models", h.CustomModels)
	r.GET("/system/support-email", h.GetSupportEmail)
	r.GET("/system/footer-data", h.GetFooterData)
	r.GET("/system/default-system-prompt", h.DefaultSystemPrompt)
	r.POST("/system/default-system-prompt", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.UpdateDefaultSystemPrompt)
	r.GET("/system/prompt-variables", h.PromptVariables)
	r.GET("/system/logo", h.Logo)
	r.POST("/system/upload-logo", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.UploadLogo)
	r.GET("/system/remove-logo", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveLogo)
	r.POST("/system/event-logs", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.EventLogs)
	r.DELETE("/system/event-logs", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.ClearEventLogs)
	r.GET("/system/is-default-logo", h.IsDefaultLogo)
	r.GET("/system/multi-user-mode", h.MultiUserMode)
	r.GET("/system/api-keys", middleware.ValidatedRequest(authSvc), h.ApiKeys)
	r.POST("/system/generate-api-key", middleware.ValidatedRequest(authSvc), h.GenerateApiKey)
	r.DELETE("/system/api-key/:id", middleware.ValidatedRequest(authSvc), h.DeleteApiKey)
	r.GET("/system/document-processing-status", middleware.ValidatedRequest(authSvc), h.DocumentProcessingStatus)
	r.GET("/system/local-files", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.LocalFiles)
	r.GET("/system/accepted-document-types", middleware.ValidatedRequest(authSvc), h.AcceptedDocumentTypes)
	r.DELETE("/system/remove-document", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveDocument)
	r.DELETE("/system/remove-documents", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveDocuments)
	r.DELETE("/system/remove-folder", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveFolder)
	r.GET("/system/custom-app-name", h.CustomAppName)
	r.GET("/system/system-vectors", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.SystemVectors)
	r.GET("/system/pfp/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.GetPfp)
	r.POST("/system/upload-pfp", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.UploadPfp)
	r.DELETE("/system/remove-pfp", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.RemovePfp)
	r.GET("/utils/metrics", h.Metrics)
	r.GET("/onboarding", h.GetOnboardingStatus)
	r.POST("/onboarding",
		middleware.ValidatedRequest(authSvc),
		h.CompleteOnboarding)
	r.GET("/system/check-token", middleware.ValidatedRequest(authSvc), h.CheckToken)
	r.GET("/system/refresh-user", middleware.ValidatedRequest(authSvc), h.RefreshUser)
	r.POST("/system/update-password", middleware.ValidatedRequest(authSvc), h.UpdatePassword)
	r.POST("/system/enable-multi-user", middleware.ValidatedRequest(authSvc), h.EnableMultiUser)
	r.POST("/system/user", middleware.ValidatedRequest(authSvc), h.UpdateOwnUser)

	// Prompt Presets
	r.GET("/system/slash-command-presets", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.ListSlashCommandPresets)
	r.POST("/system/slash-command-presets", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.CreateSlashCommandPreset)
	r.POST("/system/slash-command-presets/:slashCommandId", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.UpdateSlashCommandPreset)
	r.DELETE("/system/slash-command-presets/:slashCommandId", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.DeleteSlashCommandPreset)

	// Prompt Variables (admin only for mutations)
	r.POST("/system/prompt-variables", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.CreatePromptVariable)
	r.PUT("/system/prompt-variables/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.UpdatePromptVariable)
	r.DELETE("/system/prompt-variables/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.DeletePromptVariable)

	// Workspace Chats
	r.POST("/system/workspace-chats", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.WorkspaceChats)
	r.DELETE("/system/workspace-chats/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.DeleteWorkspaceChat)

	// Export Chats
	r.GET("/system/export-chats", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.ExportChats)

	// Validate SQL Connection
	r.POST("/system/validate-sql-connection", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.ValidateSQLConnection)
}

// ---------- Slash Command Presets ----------

func (h *SystemHandler) ListSlashCommandPresets(c *gin.Context) {
	user, _ := c.Get("user")
	u := user.(*models.User)

	presets, err := h.promptPresetSvc.ListByUser(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"presets": presets})
}

func (h *SystemHandler) CreateSlashCommandPreset(c *gin.Context) {
	user, _ := c.Get("user")
	u := user.(*models.User)

	var req struct {
		Command     string `json:"command"`
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	var uid *int
	if h.cfg.MultiUserMode {
		uid = &u.ID
	}

	preset, err := h.promptPresetSvc.Create(c.Request.Context(), uid, req.Command, req.Prompt, req.Description)
	if err != nil {
		if strings.Contains(err.Error(), "system command") {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Cannot create a preset with a command that matches a system command"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create preset"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"preset": preset})
}

func (h *SystemHandler) UpdateSlashCommandPreset(c *gin.Context) {
	user, _ := c.Get("user")
	u := user.(*models.User)

	idStr := c.Param("slashCommandId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid preset ID"})
		return
	}

	var req struct {
		Command     string `json:"command"`
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	var uid *int
	if h.cfg.MultiUserMode {
		uid = &u.ID
	}

	if err := h.promptPresetSvc.Update(c.Request.Context(), id, uid, req.Command, req.Prompt, req.Description); err != nil {
		if strings.Contains(err.Error(), "system command") {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Cannot update a preset to use a command that matches a system command"})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"message": "Preset not found"})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{})
		return
	}

	preset, _ := h.promptPresetSvc.GetByID(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"preset": preset})
}

func (h *SystemHandler) DeleteSlashCommandPreset(c *gin.Context) {
	user, _ := c.Get("user")
	u := user.(*models.User)

	idStr := c.Param("slashCommandId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid preset ID"})
		return
	}

	// Verify ownership before deleting.
	preset, err := h.promptPresetSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": "Failed to delete preset"})
		return
	}
	if h.cfg.MultiUserMode && (preset.UserID == nil || *preset.UserID != u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"message": "Failed to delete preset"})
		return
	}

	if err := h.promptPresetSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to delete preset"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---------- Prompt Variables ----------

func (h *SystemHandler) CreatePromptVariable(c *gin.Context) {
	var req struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	v, err := h.promptVariableSvc.Create(c.Request.Context(), req.Key, req.Value, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "variable": v})
}

func (h *SystemHandler) UpdatePromptVariable(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid variable ID"})
		return
	}

	var req struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if err := h.promptVariableSvc.Update(c.Request.Context(), id, req.Key, req.Value, req.Description); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Variable not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	v, _ := h.promptVariableSvc.GetByID(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"success": true, "variable": v})
}

func (h *SystemHandler) DeletePromptVariable(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid variable ID"})
		return
	}

	if err := h.promptVariableSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "System prompt variable not found or could not be deleted"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------- Workspace Chats ----------

func (h *SystemHandler) WorkspaceChats(c *gin.Context) {
	// Check if chat history viewing is disabled.
	if h.cfg.DisableViewChatHistory {
		c.String(http.StatusUnprocessableEntity, "This feature has been disabled by the administrator.")
		return
	}

	var req struct {
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Offset = 0
		req.Limit = 20
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	offset := req.Offset * req.Limit
	chats, total, err := h.wsChatSvc.ListChats(c.Request.Context(), offset, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	hasPages := total > int64((req.Offset+1)*req.Limit)
	c.JSON(http.StatusOK, gin.H{
		"chats":      chats,
		"hasPages":   hasPages,
		"totalChats": total,
	})
}

func (h *SystemHandler) DeleteWorkspaceChat(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid chat ID"})
		return
	}

	if id == -1 {
		if err := h.wsChatSvc.DeleteAllChats(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		if err := h.wsChatSvc.DeleteChat(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "error": nil})
}

// ---------- Export Chats ----------

func (h *SystemHandler) ExportChats(c *gin.Context) {
	// Check if chat history viewing is disabled.
	if h.cfg.DisableViewChatHistory {
		c.String(http.StatusUnprocessableEntity, "This feature has been disabled by the administrator.")
		return
	}

	exportType := c.Query("type")
	if exportType == "" {
		exportType = "jsonl"
	}
	chatType := c.Query("chatType")
	if chatType == "" {
		chatType = "workspace"
	}

	// Only workspace chat export is supported in this phase.
	if chatType != "workspace" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Unsupported chat type"})
		return
	}

	contentType, data, err := h.wsChatSvc.ExportChats(c.Request.Context(), exportType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, data)
}

// ---------- Validate SQL Connection ----------

func (h *SystemHandler) ValidateSQLConnection(c *gin.Context) {
	var req struct {
		Engine           string `json:"engine"`
		ConnectionString string `json:"connectionString"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Both engine and connection details are required."})
		return
	}

	if req.Engine == "" || req.ConnectionString == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Both engine and connection details are required."})
		return
	}

	result, err := validateSQLConnection(req.Engine, req.ConnectionString)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Unable to connect to %s. Please verify your connection details.", req.Engine),
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

func validateSQLConnection(engine, connectionString string) (gin.H, error) {
	switch engine {
	case "postgres":
		return validatePostgresConnection(connectionString)
	case "sqlite":
		return validateSQLiteConnection(connectionString)
	default:
		return gin.H{"success": false, "error": fmt.Sprintf("Unsupported engine: %s", engine)}, nil
	}
}

func validatePostgresConnection(connectionString string) (gin.H, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return gin.H{"success": false, "error": err.Error()}, nil
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return gin.H{"success": false, "error": err.Error()}, nil
	}
	return gin.H{"success": true}, nil
}

func validateSQLiteConnection(connectionString string) (gin.H, error) {
	db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		return gin.H{"success": false, "error": err.Error()}, nil
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return gin.H{"success": false, "error": err.Error()}, nil
	}
	return gin.H{"success": true}, nil
}
