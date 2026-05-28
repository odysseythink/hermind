package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type AdminService struct {
	db *gorm.DB
}

func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{db: db}
}

func (s *AdminService) ListUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *AdminService) DeleteUser(ctx context.Context, id int) error {
	return s.db.Delete(&models.User{}, id).Error
}

func (s *AdminService) ListWorkspaces(ctx context.Context) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := s.db.Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (s *AdminService) CreateInvite(ctx context.Context, createdBy int, workspaceIDs []int) (*models.Invite, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}
	idsJSON, _ := json.Marshal(workspaceIDs)
	idsStr := string(idsJSON)
	invite := models.Invite{
		Code:          code,
		Status:        "pending",
		CreatedBy:     createdBy,
		WorkspaceIds:  &idsStr,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if err := s.db.Create(&invite).Error; err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}
	return &invite, nil
}

func (s *AdminService) ListInvites(ctx context.Context) ([]models.Invite, error) {
	var invites []models.Invite
	if err := s.db.Order("id DESC").Find(&invites).Error; err != nil {
		return nil, err
	}
	return invites, nil
}

func (s *AdminService) DeactivateInvite(ctx context.Context, inviteID int) error {
	return s.db.Model(&models.Invite{}).Where("id = ?", inviteID).Update("status", "disabled").Error
}

// ValidRoleSelection mirrors server/utils/helpers/admin/index.js:validRoleSelection.
// Returns (valid, errorString). When updates has no "role" key, always valid.
// Admins can assign any role; managers can only assign manager/default.
func (s *AdminService) ValidRoleSelection(currentUser *models.User, updates map[string]any) (bool, string) {
	roleVal, ok := updates["role"]
	if !ok {
		return true, ""
	}
	if currentUser.Role == "admin" {
		return true, ""
	}
	if currentUser.Role == "manager" {
		newRole, _ := roleVal.(string)
		if newRole != "manager" && newRole != "default" {
			return false, "Invalid role selection for user."
		}
		return true, ""
	}
	return false, "Invalid condition for caller."
}

// ValidCanModify mirrors server/utils/helpers/admin/index.js:validCanModify.
// Admins can modify any user; managers can only modify manager/default users.
func (s *AdminService) ValidCanModify(currentUser, existingUser *models.User) (bool, string) {
	if currentUser.Role == "admin" {
		return true, ""
	}
	if currentUser.Role == "manager" {
		if existingUser.Role != "manager" && existingUser.Role != "default" {
			return false, "Cannot perform that action on user."
		}
		return true, ""
	}
	return false, "Invalid condition for caller."
}

// CanModifyAdmin mirrors server/utils/helpers/admin/index.js:canModifyAdmin.
// Prevents the last remaining admin from being demoted.
// Returns (valid, errorString).
func (s *AdminService) CanModifyAdmin(userToModify *models.User, updates map[string]any) (bool, string) {
	roleVal, ok := updates["role"]
	if !ok {
		return true, ""
	}
	if userToModify.Role != "admin" {
		return true, ""
	}
	newRole, _ := roleVal.(string)
	if newRole == "admin" {
		return true, ""
	}
	var count int64
	if err := s.db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return false, err.Error()
	}
	if count-1 <= 0 {
		return false, "No system admins will remain if you do this. Update failed."
	}
	return true, ""
}

type CreateUserInput struct {
	Username          string
	Password          string
	Role              string
	Bio               string
	DailyMessageLimit *int
}

// CreateUser returns (user, businessError, systemError). businessError is a user-facing
// validation/uniqueness message returned in JSON; systemError is a Go error.
func (s *AdminService) CreateUser(ctx context.Context, in CreateUserInput) (*models.User, string, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return nil, "Username is required.", nil
	}
	if in.Password == "" {
		return nil, "Password is required.", nil
	}
	role := in.Role
	if role != "admin" && role != "manager" && role != "default" {
		role = "default"
	}
	hash, err := utils.HashPassword(in.Password)
	if err != nil {
		return nil, "", err
	}
	u := models.User{
		Username:          utils.Ptr(username),
		Password:          hash,
		Role:              role,
		Bio:               utils.Ptr(in.Bio),
		DailyMessageLimit: in.DailyMessageLimit,
		CreatedAt:         time.Now(),
		LastUpdatedAt:     time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return nil, "A user with that username already exists", nil
		}
		return nil, "", err
	}
	return &u, "", nil
}

// UpdateUser applies a subset of writable fields. Returns (businessError, systemError).
func (s *AdminService) UpdateUser(ctx context.Context, id int, updates map[string]any) (string, error) {
	writable := map[string]bool{
		"username":          true,
		"password":          true,
		"role":              true,
		"suspended":         true,
		"bio":               true,
		"dailyMessageLimit": true,
		"pfpFilename":       true,
	}
	colMap := map[string]string{
		"dailyMessageLimit": "daily_message_limit",
		"pfpFilename":       "pfp_filename",
	}
	cleaned := map[string]any{}
	for k, v := range updates {
		if !writable[k] {
			continue
		}
		col := k
		if mapped, ok := colMap[k]; ok {
			col = mapped
		}
		if k == "password" {
			pw, _ := v.(string)
			if pw == "" {
				continue
			}
			hash, err := utils.HashPassword(pw)
			if err != nil {
				return "", err
			}
			cleaned[col] = hash
			continue
		}
		cleaned[col] = v
	}
	if len(cleaned) == 0 {
		return "No valid updates applied.", nil
	}
	cleaned["last_updated_at"] = time.Now()
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Updates(cleaned)
	if res.Error != nil {
		lower := strings.ToLower(res.Error.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return "A user with that username already exists", nil
		}
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "User not found", nil
	}
	return "", nil
}

func (s *AdminService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func generateInviteCode() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
