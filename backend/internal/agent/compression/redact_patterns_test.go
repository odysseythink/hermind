package compression

import (
	"regexp"
	"testing"
)

func TestRedactPatterns_StripsBareHex(t *testing.T) {
	patterns := RedactPatterns()
	// Find the bare-hex pattern
	var hexRe *regexp.Regexp
	for _, re := range patterns {
		if re.MatchString("deadbeef12345678") && !re.MatchString("key=abc123") {
			hexRe = re
			break
		}
	}
	if hexRe == nil {
		t.Fatal("bare-hex redact pattern not found")
	}
	input := "The hash is deadbeef12345678 and another 0xABCDEF00"
	got := hexRe.ReplaceAllString(input, "[REDACTED]")
	if got == input {
		t.Error("expected bare hex to be redacted")
	}
}

func TestRedactPatterns_StripsEmail(t *testing.T) {
	patterns := RedactPatterns()
	var emailRe *regexp.Regexp
	for _, re := range patterns {
		if re.MatchString("user@example.com") {
			emailRe = re
			break
		}
	}
	if emailRe == nil {
		t.Fatal("email redact pattern not found")
	}
	input := "Contact alice@example.com or bob+tag@company.co.uk"
	got := emailRe.ReplaceAllString(input, "[REDACTED]")
	if got == input {
		t.Error("expected email to be redacted")
	}
}

func TestRedactPatterns_KeepsAPIKeys(t *testing.T) {
	// API keys and tokens should NOT be stripped (they are conversation-relevant)
	patterns := RedactPatterns()
	for _, re := range patterns {
		if re.MatchString("sk-abc123def456") {
			t.Error("API key pattern should not match sk-... keys")
		}
		if re.MatchString("token_abc123") {
			t.Error("token pattern should not match token_... values")
		}
	}
}

func TestRedactPatterns_NotEmpty(t *testing.T) {
	patterns := RedactPatterns()
	if len(patterns) == 0 {
		t.Error("RedactPatterns() returned empty slice")
	}
}
