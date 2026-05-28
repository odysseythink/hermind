package dto

import "github.com/odysseythink/pantheon/core"

type StreamChatRequest struct {
	Message              string         `json:"message"`
	Attachments          []string       `json:"attachments,omitempty"`
	SystemPromptOverride *string        `json:"systemPromptOverride,omitempty"`
	TemperatureOverride  *float64       `json:"temperatureOverride,omitempty"`
	HistoryOverride      []core.Message `json:"-"`
}

type StreamChatResponse struct {
	UUID         string  `json:"uuid"`
	Type         string  `json:"type"` // textResponseChunk, abort, finalize, agentInitWebsocketConnection, statusResponse
	TextResponse *string `json:"textResponse,omitempty"`
	Sources      []any   `json:"sources,omitempty"`
	Close        bool    `json:"close"`
	Error        *string `json:"error,omitempty"`
	// PR-AR-4: agent handoff fields
	WebsocketUUID  *string `json:"websocketUUID,omitempty"`
	WebsocketToken *string `json:"websocketToken,omitempty"`
	Animate        bool    `json:"animate,omitempty"`
}

type ChatRequest struct {
	Message              string         `json:"message"`
	Mode                 string         `json:"mode,omitempty"`
	SessionID            string         `json:"sessionId,omitempty"`
	Reset                bool           `json:"reset,omitempty"`
	Attachments          []string       `json:"attachments,omitempty"`
	SystemPromptOverride *string        `json:"systemPromptOverride,omitempty"`
	TemperatureOverride  *float64       `json:"temperatureOverride,omitempty"`
	HistoryOverride      []core.Message `json:"-"`
}

type ChatResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	TextResponse string `json:"textResponse"`
	Sources      []any  `json:"sources"`
	Close        bool   `json:"close"`
	Error        string `json:"error,omitempty"`
}

type UpdateChatRequest struct {
	Response string `json:"response"`
	Include  *bool  `json:"include"`
}
