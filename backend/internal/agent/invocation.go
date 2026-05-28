package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrInvocationNotFound = errors.New("agent invocation not found")
	ErrInvocationClosed   = errors.New("agent invocation closed")
)

func (r *Runtime) CreateInvocation(ctx context.Context, ws *models.Workspace, user *models.User, thread *models.WorkspaceThread, prompt string) (string, error) {
	if ws == nil {
		return "", fmt.Errorf("workspace required")
	}
	inv := &models.WorkspaceAgentInvocation{
		UUID:        uuid.NewString(),
		WorkspaceID: ws.ID,
		Prompt:      prompt,
	}
	if user != nil {
		inv.UserID = &user.ID
	}
	if thread != nil {
		inv.ThreadID = &thread.ID
	}
	if err := r.deps.DB.WithContext(ctx).Create(inv).Error; err != nil {
		return "", fmt.Errorf("create invocation: %w", err)
	}
	return inv.UUID, nil
}

func (r *Runtime) GetInvocation(ctx context.Context, id string) (*models.WorkspaceAgentInvocation, error) {
	var inv models.WorkspaceAgentInvocation
	if err := r.deps.DB.WithContext(ctx).Where("uuid = ?", id).First(&inv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvocationNotFound
		}
		return nil, err
	}
	if inv.Closed {
		return nil, ErrInvocationClosed
	}
	return &inv, nil
}

func (r *Runtime) CloseInvocation(ctx context.Context, id string) error {
	return r.deps.DB.WithContext(ctx).
		Model(&models.WorkspaceAgentInvocation{}).
		Where("uuid = ?", id).
		Update("closed", true).Error
}

func (r *Runtime) DeleteInvocation(ctx context.Context, id string) error {
	return r.deps.DB.WithContext(ctx).
		Where("uuid = ?", id).
		Delete(&models.WorkspaceAgentInvocation{}).Error
}
