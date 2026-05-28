package agent

// WSFrameType constants — used in PR-AR-1 for echo; reserved for PR-AR-2/5.
const (
	FrameStatusResponse   = "statusResponse"
	FrameWSSFailure       = "wssFailure"
	FrameWaitingOnInput   = "WAITING_ON_INPUT"
	FrameToolApprovalReq  = "toolApprovalRequest"
	FrameToolApprovalResp = "toolApprovalResponse"
	FrameSetAutoApprove   = "setAutoApprove"
	FrameAwaitingFeedback = "awaitingFeedback"
	FrameUnhandled        = "__unhandled"
)

// ServerFrame is any message Go → Frontend.
type ServerFrame struct {
	Type        string `json:"type,omitempty"`
	Content     string `json:"content,omitempty"`
	Animate     bool   `json:"animate,omitempty"`
	Question    string `json:"question,omitempty"`
	RequestID   string `json:"requestId,omitempty"`   // for toolApprovalRequest
	SkillName   string `json:"skillName,omitempty"`   // for toolApprovalRequest
	Payload     any    `json:"payload,omitempty"`     // tool args, for toolApprovalRequest
	Description string `json:"description,omitempty"` // human-readable for toolApprovalRequest
	TimeoutMs   int    `json:"timeoutMs,omitempty"`   // for toolApprovalRequest
	// chat-message fields (unused in PR-AR-1, present so PR-AR-2 doesn't need a breaking change)
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	State string `json:"state,omitempty"`
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
