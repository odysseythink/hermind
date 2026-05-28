package services

import (
	"errors"
	"regexp"

	"github.com/odysseythink/hermind/backend/internal/models"
)

// usernameRegex matches Node's User.usernameRegex: lowercase start, then [a-z0-9._@-]
var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9._@-]*$`)

// ValidateUsername returns the trimmed username or an error matching Node's User.validations.username.
func ValidateUsername(raw string) (string, error) {
	if len(raw) > 32 {
		return "", errors.New("Username cannot be longer than 32 characters")
	}
	if len(raw) < 2 {
		return "", errors.New("Username must be at least 2 characters")
	}
	if !usernameRegex.MatchString(raw) {
		return "", errors.New("Username must start with a lowercase letter and only contain lowercase letters, numbers, underscores, hyphens, and periods")
	}
	return raw, nil
}

// ValidateBio matches Node's User.validations.bio: empty allowed, max 1000 chars.
func ValidateBio(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > 1000 {
		return "", errors.New("Bio cannot be longer than 1,000 characters")
	}
	return raw, nil
}

// CheckPasswordComplexity returns nil for "ok"; an error otherwise.
// P4b uses Node's lenient defaults: min=8, max=250, no character-class requirements.
// Env-driven overrides (PASSWORDMINCHAR etc.) are an explicit non-goal for P4b; track as
// follow-up if user requests it.
func CheckPasswordComplexity(pw string) error {
	if len(pw) < 8 {
		return errors.New("\"password\" length must be at least 8 characters long")
	}
	if len(pw) > 250 {
		return errors.New("\"password\" length must be less than or equal to 250 characters long")
	}
	return nil
}

// FilterUserFields returns a Node-equivalent shape (strip password, webPushSubscriptionConfig).
func FilterUserFields(u *models.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"id":                u.ID,
		"username":          u.Username,
		"pfpFilename":       u.PfpFilename,
		"role":              u.Role,
		"suspended":         u.Suspended,
		"seenRecoveryCodes": u.SeenRecoveryCodes,
		"createdAt":         u.CreatedAt,
		"lastUpdatedAt":     u.LastUpdatedAt,
		"dailyMessageLimit": u.DailyMessageLimit,
		"bio":               u.Bio,
	}
}
