package services

import (
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"ok lower", "alice", true},
		{"ok dots/dashes", "a.b-c_d@e", true},
		{"too short", "a", false},
		{"too long", "a23456789012345678901234567890123", false}, // 33 chars
		{"starts with digit", "1abc", false},
		{"upper case", "Alice", false},
		{"space", "al ice", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateUsername(tc.in)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateUsername(%q) ok=%v, err=%v", tc.in, tc.ok, err)
			}
		})
	}
}

func TestValidateBio(t *testing.T) {
	if _, err := ValidateBio(""); err != nil {
		t.Fatal("empty bio should be ok")
	}
	if _, err := ValidateBio("hi"); err != nil {
		t.Fatal("short bio should be ok")
	}
	big := make([]byte, 1001)
	for i := range big {
		big[i] = 'a'
	}
	if _, err := ValidateBio(string(big)); err == nil {
		t.Fatal("1001-char bio should error")
	}
}

func TestCheckPasswordComplexity(t *testing.T) {
	if err := CheckPasswordComplexity("short"); err == nil {
		t.Fatal("len<8 should error")
	}
	if err := CheckPasswordComplexity("abcdefgh"); err != nil {
		t.Fatalf("len=8 should pass, got %v", err)
	}
	long := make([]byte, 251)
	for i := range long {
		long[i] = 'a'
	}
	if err := CheckPasswordComplexity(string(long)); err == nil {
		t.Fatal("len>250 should error")
	}
}

func TestFilterUserFields(t *testing.T) {
	u := &models.User{
		ID:                        7,
		Username:                  utils.Ptr("alice"),
		Password:                  "SECRET",
		WebPushSubscriptionConfig: utils.Ptr("{...}"),
		Role:                      "default",
		Bio:                       utils.Ptr("hi"),
	}
	out := FilterUserFields(u)
	if _, has := out["password"]; has {
		t.Fatal("password must be stripped")
	}
	if _, has := out["webPushSubscriptionConfig"]; has {
		t.Fatal("webPushSubscriptionConfig must be stripped")
	}
	if out["id"] != 7 || out["role"] != "default" {
		t.Fatalf("unexpected output: %+v", out)
	}
}
