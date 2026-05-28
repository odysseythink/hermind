package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
	enc *utils.EncryptionManager
}

func NewAuthService(db *gorm.DB, cfg *config.Config, enc *utils.EncryptionManager) *AuthService {
	return &AuthService{db: db, cfg: cfg, enc: enc}
}

func (s *AuthService) IsAuthEnabled() bool {
	return s.cfg.AuthToken != "" && s.cfg.JWTSecret != ""
}

func (s *AuthService) ValidateToken(ctx context.Context, tokenStr string) (*models.User, error) {
	if !s.cfg.MultiUserMode {
		return s.validateSingleUserToken(tokenStr)
	}
	return s.validateMultiUserSession(tokenStr)
}

func (s *AuthService) validateSingleUserToken(tokenStr string) (*models.User, error) {
	claims, err := utils.ParseJWT(s.cfg.JWTSecret, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	pVal, ok := claims["p"].(string)
	if !ok {
		return nil, fmt.Errorf("missing payload")
	}
	decrypted, err := s.enc.Decrypt(pVal)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}
	if s.cfg.AuthToken != decrypted {
		return nil, fmt.Errorf("token mismatch")
	}
	return &models.User{ID: 0, Username: utils.Ptr("admin"), Role: "admin"}, nil
}

func (s *AuthService) validateMultiUserSession(tokenStr string) (*models.User, error) {
	claims, err := utils.ParseJWT(s.cfg.JWTSecret, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}
	userID, ok := claims["userId"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing userId")
	}
	var user models.User
	if err := s.db.First(&user, int(userID)).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	var user models.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !utils.CheckPassword(req.Password, user.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return &dto.LoginResponse{User: user, Token: token, Message: "ok"}, nil
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.LoginResponse, error) {
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user := models.User{Username: utils.Ptr(req.Username), Password: hash, Role: "default"}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	return &dto.LoginResponse{User: user, Token: token, Message: "ok"}, nil
}

func (s *AuthService) GenerateRecoveryCodes(ctx context.Context, userID int) ([]string, error) {
	var plaintext []string
	for i := 0; i < 4; i++ {
		code := uuid.New().String()
		plaintext = append(plaintext, code)
		hash, err := utils.HashPassword(code)
		if err != nil {
			return nil, fmt.Errorf("hash recovery code: %w", err)
		}
		rc := models.RecoveryCode{
			UserID:    userID,
			CodeHash:  hash,
			CreatedAt: time.Now(),
		}
		if err := s.db.WithContext(ctx).Create(&rc).Error; err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("seen_recovery_codes", true).Error; err != nil {
		return nil, fmt.Errorf("update seen_recovery_codes: %w", err)
	}
	return plaintext, nil
}

func (s *AuthService) RequestTokenMultiUser(ctx context.Context, username, password string, eventLogSvc *EventLogService) (*dto.LoginResponse, []string, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_invalid_username", map[string]any{"username": username}, nil)
		return nil, nil, fmt.Errorf("invalid credentials")
	}
	if !utils.CheckPassword(password, user.Password) {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_invalid_password", map[string]any{"username": username}, &user.ID)
		return nil, nil, fmt.Errorf("invalid credentials")
	}
	if user.Suspended == 1 {
		_ = eventLogSvc.LogEvent(ctx, "failed_login_account_suspended", map[string]any{"username": username}, &user.ID)
		return nil, nil, fmt.Errorf("account suspended")
	}
	_ = eventLogSvc.LogEvent(ctx, "login_event", map[string]any{"username": username}, &user.ID)

	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"userId": user.ID}, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("generate token: %w", err)
	}

	var recoveryCodes []string
	if user.SeenRecoveryCodes == nil || !*user.SeenRecoveryCodes {
		recoveryCodes, err = s.GenerateRecoveryCodes(ctx, user.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("generate recovery codes: %w", err)
		}
	}

	return &dto.LoginResponse{User: user, Token: token, Message: "ok"}, recoveryCodes, nil
}

func (s *AuthService) CreateSingleUserToken(ctx context.Context) (string, error) {
	encrypted, err := s.enc.Encrypt(s.cfg.AuthToken)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}
	token, err := utils.GenerateJWT(s.cfg.JWTSecret, map[string]any{"p": encrypted}, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) RecoverAccount(ctx context.Context, username string, recoveryCodes []string) (string, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return "", fmt.Errorf("invalid recovery codes")
	}

	var codes []models.RecoveryCode
	if err := s.db.Where("user_id = ?", user.ID).Find(&codes).Error; err != nil {
		return "", fmt.Errorf("invalid recovery codes")
	}
	if len(codes) < 4 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	seen := make(map[string]bool)
	var uniqueCodes []string
	for _, code := range recoveryCodes {
		trimmed := code
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		uniqueCodes = append(uniqueCodes, trimmed)
		if len(uniqueCodes) >= 2 {
			break
		}
	}
	if len(uniqueCodes) != 2 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	valid := 0
	for _, code := range uniqueCodes {
		for _, rc := range codes {
			if utils.CheckPassword(code, rc.CodeHash) {
				valid++
				break
			}
		}
	}
	if valid != 2 {
		return "", fmt.Errorf("invalid recovery codes")
	}

	token := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&token).Error; err != nil {
		return "", fmt.Errorf("create reset token: %w", err)
	}
	return token.Token, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword, confirmPassword string) error {
	if newPassword != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}
	if newPassword == "" {
		return fmt.Errorf("invalid password")
	}

	var resetToken models.PasswordResetToken
	if err := s.db.Where("token = ?", token).First(&resetToken).Error; err != nil {
		return fmt.Errorf("invalid reset token")
	}
	if time.Now().After(resetToken.ExpiresAt) {
		return fmt.Errorf("invalid reset token")
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", resetToken.UserID).Update("password", hash).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	s.db.Where("user_id = ?", resetToken.UserID).Delete(&models.PasswordResetToken{})
	s.db.Where("user_id = ?", resetToken.UserID).Delete(&models.RecoveryCode{})

	return nil
}

func (s *AuthService) UpdatePfp(ctx context.Context, userID int, pfpFilename *string) error {
	return s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("pfp_filename", pfpFilename).Error
}

func (s *AuthService) RotateCredentials(ctx context.Context, sysSvc *SystemService, usePassword bool, newPassword string) error {
	if !usePassword {
		s.cfg.AuthToken = ""
		s.cfg.JWTSecret = ""
		if err := sysSvc.SetSetting(ctx, "auth_token", ""); err != nil {
			return err
		}
		if err := sysSvc.SetSetting(ctx, "jwt_secret", ""); err != nil {
			return err
		}
		return nil
	}
	if err := CheckPasswordComplexity(newPassword); err != nil {
		return err
	}
	newSecret := uuid.New().String()
	s.cfg.AuthToken = newPassword
	s.cfg.JWTSecret = newSecret
	if err := sysSvc.SetSetting(ctx, "auth_token", newPassword); err != nil {
		return err
	}
	if err := sysSvc.SetSetting(ctx, "jwt_secret", newSecret); err != nil {
		return err
	}
	return nil
}

func (s *AuthService) EnableMultiUser(
	ctx context.Context,
	adminSvc *AdminService,
	sysSvc *SystemService,
	username, password string,
) (*models.User, string, error) {
	if s.cfg.MultiUserMode {
		return nil, "Multi-user mode is already enabled.", nil
	}
	if _, err := ValidateUsername(username); err != nil {
		return nil, err.Error(), nil
	}
	if err := CheckPasswordComplexity(password); err != nil {
		return nil, err.Error(), nil
	}

	user, bizErr, sysErr := adminSvc.CreateUser(ctx, CreateUserInput{
		Username: username, Password: password, Role: "admin",
	})
	if sysErr != nil {
		return nil, "", sysErr
	}
	if bizErr != "" {
		return nil, bizErr, nil
	}

	if err := sysSvc.SetSetting(ctx, "multi_user_mode", "true"); err != nil {
		// best-effort rollback
		s.db.Delete(&models.User{}, user.ID)
		return nil, "", err
	}
	if s.cfg.JWTSecret == "" {
		newSecret := uuid.New().String()
		s.cfg.JWTSecret = newSecret
		_ = sysSvc.SetSetting(ctx, "jwt_secret", newSecret)
	}
	s.cfg.MultiUserMode = true
	return user, "", nil
}

func (s *AuthService) UpdateOwnProfile(
	ctx context.Context,
	user *models.User,
	newUsername, newPassword, newBio *string,
) (string, error) {
	if user == nil || user.ID == 0 {
		return "Invalid user ID", nil
	}
	updates := map[string]any{}
	if newUsername != nil {
		cur := ""
		if user.Username != nil {
			cur = *user.Username
		}
		if *newUsername != cur {
			validated, err := ValidateUsername(*newUsername)
			if err != nil {
				return err.Error(), nil
			}
			updates["username"] = validated
		}
	}
	if newPassword != nil && *newPassword != "" {
		if err := CheckPasswordComplexity(*newPassword); err != nil {
			return err.Error(), nil
		}
		hash, err := utils.HashPassword(*newPassword)
		if err != nil {
			return "", err
		}
		updates["password"] = hash
	}
	if newBio != nil && *newBio != "" {
		validated, err := ValidateBio(*newBio)
		if err != nil {
			return err.Error(), nil
		}
		updates["bio"] = validated
	}
	if len(updates) == 0 {
		return "No updates provided", nil
	}
	updates["last_updated_at"] = time.Now()
	res := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
	if res.Error != nil {
		lower := strings.ToLower(res.Error.Error())
		if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
			return "A user with that username already exists", nil
		}
		return "", res.Error
	}
	return "", nil
}

func (s *AuthService) GetInvite(ctx context.Context, code string) (*models.Invite, error) {
	var invite models.Invite
	if err := s.db.Where("code = ? AND status = ?", code, "pending").First(&invite).Error; err != nil {
		return nil, fmt.Errorf("invite not found or is invalid")
	}
	return &invite, nil
}

func (s *AuthService) AcceptInvite(ctx context.Context, code, username, password string) error {
	var invite models.Invite
	if err := s.db.Where("code = ? AND status = ?", code, "pending").First(&invite).Error; err != nil {
		return fmt.Errorf("invite not found or is invalid")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		Username: utils.Ptr(username),
		Password: hash,
		Role:     "default",
	}
	if err := s.db.Create(&user).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	invite.Status = "claimed"
	invite.ClaimedBy = &user.ID
	if err := s.db.Save(&invite).Error; err != nil {
		return fmt.Errorf("update invite: %w", err)
	}

	if invite.WorkspaceIds != nil && *invite.WorkspaceIds != "" {
		var ids []int
		if err := json.Unmarshal([]byte(*invite.WorkspaceIds), &ids); err == nil && len(ids) > 0 {
			for _, wid := range ids {
				wu := models.WorkspaceUser{
					WorkspaceID:   wid,
					UserID:        user.ID,
					Role:          "default",
					CreatedAt:     time.Now(),
					LastUpdatedAt: time.Now(),
				}
				s.db.Create(&wu)
			}
		}
	}

	return nil
}
