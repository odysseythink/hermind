// cli/ui/skin_test.go
package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSkinHasColors(t *testing.T) {
	s := DefaultSkin()
	assert.Equal(t, "default", s.Name)
	assert.NotEmpty(t, s.Accent)
	assert.NotEmpty(t, s.Error)
}

func TestMinimalSkinHasNoColors(t *testing.T) {
	s := MinimalSkin()
	assert.Equal(t, "minimal", s.Name)
}

func TestDetectSkinFromTruecolor(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM", "xterm-256color")
	s := DetectSkin()
	assert.Equal(t, "default", s.Name)
}

func TestDetectSkinFromDumbTerminal(t *testing.T) {
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM", "dumb")
	s := DetectSkin()
	assert.Equal(t, "minimal", s.Name)
}
