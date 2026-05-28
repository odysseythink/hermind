package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/scheduler"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type ScheduledJobsHandler struct {
	svc     *services.ScheduledJobService
	sched   *scheduler.JobScheduler
	contSvc *services.ScheduledJobContinueService
}

func NewScheduledJobsHandler(svc *services.ScheduledJobService, sched *scheduler.JobScheduler, contSvc *services.ScheduledJobContinueService) *ScheduledJobsHandler {
	return &ScheduledJobsHandler{svc: svc, sched: sched, contSvc: contSvc}
}

type createReq struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Tools    string `json:"tools"`
	Schedule string `json:"schedule"`
}

func (h *ScheduledJobsHandler) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	job, err := h.svc.Create(c.Request.Context(), services.ScheduledJobInput{
		Name: req.Name, Prompt: req.Prompt, Tools: req.Tools, Schedule: req.Schedule,
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidCron) {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cron"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	_ = h.sched.SyncJob(c.Request.Context(), job.ID)
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *ScheduledJobsHandler) List(c *gin.Context) {
	jobs, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

type updateReq struct {
	Name     *string `json:"name"`
	Prompt   *string `json:"prompt"`
	Tools    *string `json:"tools"`
	Schedule *string `json:"schedule"`
	Enabled  *bool   `json:"enabled"`
}

func (h *ScheduledJobsHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	job, err := h.svc.Update(c.Request.Context(), id, services.UpdateJobInput{
		Name: req.Name, Prompt: req.Prompt, Tools: req.Tools,
		Schedule: req.Schedule, Enabled: req.Enabled,
	})
	if err != nil {
		if errors.Is(err, services.ErrJobNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "job not found"})
			return
		}
		if errors.Is(err, services.ErrInvalidCron) {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid cron"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	_ = h.sched.SyncJob(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *ScheduledJobsHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	h.sched.RemoveJob(c.Request.Context(), id)
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ScheduledJobsHandler) Trigger(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	if _, err := h.svc.Get(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrJobNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	run, err := h.sched.EnqueueOnce(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if run == nil {
		c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "already in flight"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *ScheduledJobsHandler) ListRuns(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	runs, err := h.svc.ListRuns(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func (h *ScheduledJobsHandler) KillRun(c *gin.Context) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	killed, err := h.sched.KillRun(c.Request.Context(), runID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "run not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": killed})
}

func (h *ScheduledJobsHandler) MarkRunRead(c *gin.Context) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	if err := h.svc.MarkRunRead(c.Request.Context(), runID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ScheduledJobsHandler) Continue(c *gin.Context) {
	runID, err := strconv.Atoi(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "bad id"})
		return
	}
	ws, thr, err := h.contSvc.ContinueInThread(c.Request.Context(), runID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "run not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": ws, "thread": thr})
}

type ToolCatalogItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	RequiresSetup bool   `json:"requiresSetup,omitempty"`
}

type ToolCatalogCategory struct {
	Category string            `json:"category"`
	Name     string            `json:"name"`
	Items    []ToolCatalogItem `json:"items"`
}

func (h *ScheduledJobsHandler) ListTools(c *gin.Context) {
	// Static default skills first (always present).
	cats := []ToolCatalogCategory{
		{
			Category: "agent-skills", Name: "Agent Skills",
			Items: []ToolCatalogItem{
				{ID: "rag-memory", Name: "RAG Memory", Description: "Recall and cite information from embedded documents"},
				{ID: "document-summarizer", Name: "Document Summarizer", Description: "Summarize documents in the workspace"},
				{ID: "web-scraping", Name: "Web Scraping", Description: "Scrape content from web pages"},
				{ID: "create-chart", Name: "Create Charts", Description: "Generate data visualization charts"},
				{ID: "web-browsing", Name: "Web Browsing", Description: "Search and browse the web"},
				{ID: "sql-agent", Name: "SQL Agent", Description: "Query connected SQL databases"},
				{ID: "filesystem-agent", Name: "Filesystem"},
				{ID: "create-files-agent", Name: "Create Files"},
			},
		},
	}
	// MCP servers — left as a TODO for the engineer to populate from MCPService
	// if available. The shape:
	//   {Category: "mcp-servers", Name: "MCP Servers", Items: [{ID: "@@mcp_" + serverName, Name: ..., Description: ...}]}
	c.JSON(http.StatusOK, gin.H{"categories": cats})
}

func RegisterScheduledJobsRoutes(r *gin.RouterGroup, svc *services.ScheduledJobService, sched *scheduler.JobScheduler, contSvc *services.ScheduledJobContinueService, authSvc *services.AuthService) {
	h := NewScheduledJobsHandler(svc, sched, contSvc)
	g := r.Group("/scheduled-jobs", middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}))
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PATCH("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/trigger", h.Trigger)
	g.GET("/:id/runs", h.ListRuns)
	g.POST("/runs/:runId/kill", h.KillRun)
	g.POST("/runs/:runId/read", h.MarkRunRead)
	g.POST("/runs/:runId/continue", h.Continue)
	g.GET("/tools", h.ListTools)
}
