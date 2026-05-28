package dto

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User    any    `json:"user"`
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RequestTokenResponse struct {
	Token         string   `json:"token"`
	Valid         bool     `json:"valid"`
	User          any      `json:"user,omitempty"`
	Message       *string  `json:"message,omitempty"`
	RecoveryCodes []string `json:"recoveryCodes,omitempty"`
}

type RecoverAccountRequest struct {
	Username      string   `json:"username"`
	RecoveryCodes []string `json:"recoveryCodes"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type UpdatePasswordRequest struct {
	UsePassword bool   `json:"usePassword"`
	NewPassword string `json:"newPassword"`
}

type RequestTokenMultiUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AcceptInviteRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
