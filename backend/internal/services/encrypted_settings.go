package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/pkg/utils"
	"gorm.io/gorm"
)

const EncryptedPrefix = "enc:"

// GetSecretField reads SystemSetting[settingKey] as JSON, extracts the string
// field jsonField, and decrypts it if it carries the "enc:" prefix.
func (s *SystemService) GetSecretField(ctx context.Context, settingKey, jsonField string, enc *utils.EncryptionManager) (string, error) {
	raw, err := s.GetSetting(ctx, settingKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if raw == "" {
		return "", nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", fmt.Errorf("setting %q is not JSON: %w", settingKey, err)
	}
	v, _ := obj[jsonField].(string)
	if strings.HasPrefix(v, EncryptedPrefix) {
		plain, err := enc.Decrypt(strings.TrimPrefix(v, EncryptedPrefix))
		if err != nil {
			return "", fmt.Errorf("decrypt %s.%s: %w", settingKey, jsonField, err)
		}
		return plain, nil
	}
	return v, nil
}

// SetSecretField writes plaintext into SystemSetting[settingKey] JSON's
// jsonField, encrypted with the "enc:" prefix. Preserves other fields.
func (s *SystemService) SetSecretField(ctx context.Context, settingKey, jsonField, plaintext string, enc *utils.EncryptionManager) error {
	raw, _ := s.GetSetting(ctx, settingKey)
	obj := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &obj)
	}
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		return err
	}
	obj[jsonField] = EncryptedPrefix + ciphertext
	data, _ := json.Marshal(obj)
	return s.SetSetting(ctx, settingKey, string(data))
}
