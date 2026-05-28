package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

// WorkspaceChatWithData mirrors Node.js WorkspaceChats.whereWithData result.
type WorkspaceChatWithData struct {
	models.WorkspaceChat
	Workspace struct {
		Name string  `json:"name"`
		Slug *string `json:"slug"`
	} `json:"workspace"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

// ChatResponse is the JSON structure stored in WorkspaceChat.Response.
type ChatResponse struct {
	Text        string       `json:"text"`
	Sources     []Source     `json:"sources"`
	Attachments []Attachment `json:"attachments"`
}

type Source struct {
	Text string `json:"text"`
}

type Attachment struct {
	ContentString string `json:"contentString"`
	Mime          string `json:"mime"`
}

type WorkspaceChatService struct {
	db *gorm.DB
}

func NewWorkspaceChatService(db *gorm.DB) *WorkspaceChatService {
	return &WorkspaceChatService{db: db}
}

// ListChats returns paginated workspace chats with workspace and user data.
func (s *WorkspaceChatService) ListChats(ctx context.Context, offset, limit int) ([]WorkspaceChatWithData, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&models.WorkspaceChat{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var chats []models.WorkspaceChat
	if err := s.db.WithContext(ctx).Order("id desc").Offset(offset).Limit(limit).Find(&chats).Error; err != nil {
		return nil, 0, err
	}

	// Collect workspace and user IDs for batch lookup.
	workspaceIDs := make(map[int]struct{})
	userIDs := make(map[int]struct{})
	for _, c := range chats {
		workspaceIDs[c.WorkspaceID] = struct{}{}
		if c.UserID != nil {
			userIDs[*c.UserID] = struct{}{}
		}
	}

	// Batch query workspaces.
	workspaceMap := make(map[int]models.Workspace)
	if len(workspaceIDs) > 0 {
		ids := make([]int, 0, len(workspaceIDs))
		for id := range workspaceIDs {
			ids = append(ids, id)
		}
		var workspaces []models.Workspace
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&workspaces).Error; err == nil {
			for _, w := range workspaces {
				workspaceMap[w.ID] = w
			}
		}
	}

	// Batch query users.
	userMap := make(map[int]models.User)
	if len(userIDs) > 0 {
		ids := make([]int, 0, len(userIDs))
		for id := range userIDs {
			ids = append(ids, id)
		}
		var users []models.User
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}

	// Assemble results.
	result := make([]WorkspaceChatWithData, len(chats))
	for i, c := range chats {
		result[i] = WorkspaceChatWithData{WorkspaceChat: c}

		if w, ok := workspaceMap[c.WorkspaceID]; ok {
			result[i].Workspace.Name = w.Name
			result[i].Workspace.Slug = &w.Slug
		} else {
			result[i].Workspace.Name = "deleted workspace"
		}

		if c.UserID != nil {
			if u, ok := userMap[*c.UserID]; ok && u.Username != nil {
				result[i].User.Username = *u.Username
			} else {
				result[i].User.Username = "unknown user"
			}
		} else if c.APISessionID != nil {
			result[i].User.Username = "API"
		} else {
			result[i].User.Username = "unknown user"
		}
	}

	return result, total, nil
}

// CountChats returns the total number of workspace chats.
func (s *WorkspaceChatService) CountChats(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.WithContext(ctx).Model(&models.WorkspaceChat{}).Count(&total).Error
	return total, err
}

// DeleteChat deletes a workspace chat by ID.
func (s *WorkspaceChatService) DeleteChat(ctx context.Context, id int) error {
	return s.db.WithContext(ctx).Delete(&models.WorkspaceChat{}, id).Error
}

// DeleteAllChats deletes all workspace chats.
func (s *WorkspaceChatService) DeleteAllChats(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&models.WorkspaceChat{}).Error
}

// ExportChats exports workspace chats in the requested format.
// Supported formats: "csv", "jsonl". Returns content-type and data.
func (s *WorkspaceChatService) ExportChats(ctx context.Context, format string) (string, []byte, error) {
	var chats []models.WorkspaceChat
	if err := s.db.WithContext(ctx).Order("id asc").Find(&chats).Error; err != nil {
		return "", nil, err
	}

	switch format {
	case "csv":
		return s.exportCSV(chats)
	case "jsonl":
		return s.exportJSONL(ctx, chats)
	case "json":
		return s.exportJSON(chats)
	default:
		return s.exportJSONL(ctx, chats)
	}
}

func (s *WorkspaceChatService) exportCSV(chats []models.WorkspaceChat) (string, []byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "workspace", "prompt", "response", "sent_at", "username", "rating"})

	for _, c := range chats {
		respText := extractResponseText(c.Response)
		rating := "--"
		if c.FeedbackScore != nil {
			if *c.FeedbackScore {
				rating = "GOOD"
			} else {
				rating = "BAD"
			}
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", c.ID),
			fmt.Sprintf("%d", c.WorkspaceID),
			c.Prompt,
			respText,
			c.CreatedAt.Format(time.RFC3339),
			"",
			rating,
		})
	}
	w.Flush()
	return "text/csv", []byte(buf.String()), nil
}

func (s *WorkspaceChatService) exportJSON(chats []models.WorkspaceChat) (string, []byte, error) {
	type exportItem struct {
		ID       int    `json:"id"`
		Prompt   string `json:"prompt"`
		Response string `json:"response"`
		SentAt   string `json:"sent_at"`
	}
	items := make([]exportItem, len(chats))
	for i, c := range chats {
		items[i] = exportItem{
			ID:       c.ID,
			Prompt:   c.Prompt,
			Response: extractResponseText(c.Response),
			SentAt:   c.CreatedAt.Format(time.RFC3339),
		}
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", nil, err
	}
	return "application/json", data, nil
}

func (s *WorkspaceChatService) exportJSONL(ctx context.Context, chats []models.WorkspaceChat) (string, []byte, error) {
	// Collect workspace IDs for prompt lookup.
	workspaceIDs := make(map[int]struct{})
	for _, c := range chats {
		workspaceIDs[c.WorkspaceID] = struct{}{}
	}

	workspaceMap := make(map[int]*models.Workspace)
	if len(workspaceIDs) > 0 {
		ids := make([]int, 0, len(workspaceIDs))
		for id := range workspaceIDs {
			ids = append(ids, id)
		}
		var workspaces []models.Workspace
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&workspaces).Error; err == nil {
			for i := range workspaces {
				workspaceMap[workspaces[i].ID] = &workspaces[i]
			}
		}
	}

	// Group by workspaceID.
	type message struct {
		Role    string `json:"role"`
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			Image string `json:"image,omitempty"`
		} `json:"content"`
	}

	type workspaceThread struct {
		Messages []message `json:"messages"`
	}

	groups := make(map[int]*workspaceThread)
	for _, c := range chats {
		if _, ok := groups[c.WorkspaceID]; !ok {
			// System message with workspace prompt.
			ws := workspaceMap[c.WorkspaceID]
			systemPrompt := "Given the following conversation, relevant context, and a follow up question, reply with an answer to the current question the user is asking."
			if ws != nil && ws.OpenAiPrompt != nil {
				systemPrompt = *ws.OpenAiPrompt
			}
			groups[c.WorkspaceID] = &workspaceThread{
				Messages: []message{
					{
						Role: "system",
						Content: []struct {
							Type  string `json:"type"`
							Text  string `json:"text,omitempty"`
							Image string `json:"image,omitempty"`
						}{
							{Type: "text", Text: systemPrompt},
						},
					},
				},
			}
		}

		resp := parseChatResponse(c.Response)

		// User message.
		userContent := []struct {
			Type  string `json:"type"`
			Text  string `json:"text,omitempty"`
			Image string `json:"image,omitempty"`
		}{
			{Type: "text", Text: c.Prompt},
		}
		for _, att := range resp.Attachments {
			dataURL := att.ContentString
			if !strings.HasPrefix(dataURL, "data:") {
				dataURL = fmt.Sprintf("data:%s;base64,%s", att.Mime, att.ContentString)
			}
			userContent = append(userContent, struct {
				Type  string `json:"type"`
				Text  string `json:"text,omitempty"`
				Image string `json:"image,omitempty"`
			}{Type: "image", Image: dataURL})
		}
		groups[c.WorkspaceID].Messages = append(groups[c.WorkspaceID].Messages, message{
			Role:    "user",
			Content: userContent,
		})

		// Assistant message.
		groups[c.WorkspaceID].Messages = append(groups[c.WorkspaceID].Messages, message{
			Role: "assistant",
			Content: []struct {
				Type  string `json:"type"`
				Text  string `json:"text,omitempty"`
				Image string `json:"image,omitempty"`
			}{
				{Type: "text", Text: resp.Text},
			},
		})
	}

	// Serialize each group as a JSON line.
	var lines []string
	for _, group := range groups {
		b, err := json.Marshal(group)
		if err != nil {
			continue
		}
		lines = append(lines, string(b))
	}

	return "application/jsonl", []byte(strings.Join(lines, "\n")), nil
}

func extractResponseText(response string) string {
	resp := parseChatResponse(response)
	return resp.Text
}

func parseChatResponse(response string) ChatResponse {
	var resp ChatResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		// If not valid JSON, treat entire response as text.
		resp.Text = response
	}
	return resp
}
