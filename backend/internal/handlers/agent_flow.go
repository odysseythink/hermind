package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type AgentFlowHandler struct {
	flowSvc *services.AgentFlowService
}

func NewAgentFlowHandler(flowSvc *services.AgentFlowService) *AgentFlowHandler {
	return &AgentFlowHandler{flowSvc: flowSvc}
}

func (h *AgentFlowHandler) ListFlows(c *gin.Context) {
	flows, err := h.flowSvc.ListFlows()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error(), "flows": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flows": flows})
}

func (h *AgentFlowHandler) GetFlow(c *gin.Context) {
	flowUUID := c.Param("uuid")
	flow, err := h.flowSvc.LoadFlow(flowUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Flow not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow": flow})
}

func (h *AgentFlowHandler) SaveFlow(c *gin.Context) {
	var req struct {
		Name   string              `json:"name"`
		Config services.FlowConfig `json:"config"`
		UUID   string              `json:"uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error(), "flow": nil})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required", "flow": nil})
		return
	}
	id, err := h.flowSvc.SaveFlow(req.Name, req.Config, req.UUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "flow": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow": gin.H{"uuid": id, "name": req.Name, "config": req.Config}})
}

func (h *AgentFlowHandler) DeleteFlow(c *gin.Context) {
	flowUUID := c.Param("uuid")
	if err := h.flowSvc.DeleteFlow(flowUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AgentFlowHandler) ToggleFlow(c *gin.Context) {
	flowUUID := c.Param("uuid")
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	flow, err := h.flowSvc.LoadFlow(flowUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Flow not found"})
		return
	}
	flow.Config.Active = req.Active
	_, err = h.flowSvc.SaveFlow(flow.Name, flow.Config, flowUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "flow": flow})
}

func RegisterAgentFlowRoutes(r *gin.RouterGroup, flowSvc *services.AgentFlowService, authSvc *services.AuthService) {
	h := NewAgentFlowHandler(flowSvc)
	r.GET("/agent-flows/list",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListFlows)
	r.GET("/agent-flows/:uuid",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.GetFlow)
	r.POST("/agent-flows/save",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.SaveFlow)
	r.DELETE("/agent-flows/:uuid",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteFlow)
	r.POST("/agent-flows/:uuid/toggle",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ToggleFlow)
}
