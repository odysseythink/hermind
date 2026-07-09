package agent

import "encoding/json"

// WSFrameType constants — used in PR-AR-1 for echo; reserved for PR-AR-2/5.
const (
	FrameStatusResponse   = "statusResponse"
	FrameWSSFailure       = "wssFailure"
	FrameWaitingOnInput   = "WAITING_ON_INPUT"
	FrameToolApprovalReq  = "toolApprovalRequest"
	FrameToolApprovalResp = "toolApprovalResponse"
	FrameSetAutoApprove   = "setAutoApprove"
	FrameAwaitingFeedback = "awaitingFeedback"
	FrameUnhandled            = "__unhandled"
	FrameReportStreamEvent    = "reportStreamEvent"
)

// ServerFrame is any message Go → Frontend.
type ServerFrame struct {
	Type    string `json:"type,omitempty"`
	Content string `json:"-"`
	// ContentObj carries an object payload for frames that need structured
	// content (e.g., reportStreamEvent citations). When non-nil, it wins
	// over Content during JSON marshaling.
	ContentObj  any    `json:"content,omitempty"`
	Animate     bool   `json:"animate,omitempty"`
	Question    string `json:"question,omitempty"`
	RequestID   string `json:"requestId,omitempty"`   // for toolApprovalRequest
	SkillName   string `json:"skillName,omitempty"`   // for toolApprovalRequest
	Payload     any    `json:"payload,omitempty"`     // tool args, for toolApprovalRequest
	Description string `json:"description,omitempty"` // human-readable for toolApprovalRequest
	TimeoutMs   int    `json:"timeoutMs,omitempty"`   // for toolApprovalRequest
	// UUID is the assistant message UUID; assigned by bridge.go.
	UUID  string `json:"uuid,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	State string `json:"state,omitempty"`
}

func (f ServerFrame) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	if f.Type != "" {
		m["type"] = f.Type
	}
	if f.ContentObj != nil {
		m["content"] = f.ContentObj
	} else if f.Content != "" {
		m["content"] = f.Content
	}
	if f.Animate {
		m["animate"] = f.Animate
	}
	if f.Question != "" {
		m["question"] = f.Question
	}
	if f.RequestID != "" {
		m["requestId"] = f.RequestID
	}
	if f.SkillName != "" {
		m["skillName"] = f.SkillName
	}
	if f.Payload != nil {
		m["payload"] = f.Payload
	}
	if f.Description != "" {
		m["description"] = f.Description
	}
	if f.TimeoutMs != 0 {
		m["timeoutMs"] = f.TimeoutMs
	}
	if f.UUID != "" {
		m["uuid"] = f.UUID
	}
	if f.From != "" {
		m["from"] = f.From
	}
	if f.To != "" {
		m["to"] = f.To
	}
	if f.State != "" {
		m["state"] = f.State
	}
	return json.Marshal(m)
}

func (f *ServerFrame) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion with UnmarshalJSON.
	type alias ServerFrame
	raw := struct {
		*alias
		Content json.RawMessage `json:"content,omitempty"`
	}{
		alias: (*alias)(f),
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// If "content" is present, try as string first, then fall back to object.
	if len(raw.Content) > 0 {
		var s string
		if err := json.Unmarshal(raw.Content, &s); err == nil {
			f.Content = s
		} else {
			f.ContentObj = raw.Content
		}
	}
	return nil
}

// ClientFrame is any message Frontend → Go.
type ClientFrame struct {
	Type        string `json:"type,omitempty"`
	Feedback    string `json:"feedback,omitempty"`
	Attachments []any  `json:"attachments,omitempty"`
	RequestID   string `json:"requestId,omitempty"`
	Approved    bool   `json:"approved,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"` // for setAutoApprove
}
