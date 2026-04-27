package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSkill drops a minimal SKILL.md under <root>/skills/<category>/<name>/.
func writeSkill(t *testing.T, root, category, name, description string) {
	t.Helper()
	dir := filepath.Join(root, "skills", category, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := "---\nname: " + name + "\ndescription: " + description + "\n---\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644))
}

func newSkillsServer(t *testing.T, instanceRoot string, disabled []string) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Skills.Disabled = disabled
	s, err := NewServer(&ServerOpts{
		Config:       cfg,
		InstanceRoot: instanceRoot,
	})
	require.NoError(t, err)
	return s
}

func getSkills(t *testing.T, s *Server) []SkillDTO {
	t.Helper()
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/skills", nil))
	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())
	var resp SkillsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp.Skills
}

func TestSkillsList_EmptyHome_NoDisabled(t *testing.T) {
	root := t.TempDir()
	s := newSkillsServer(t, root, nil)
	got := getSkills(t, s)
	assert.Empty(t, got, "no skills on disk and none disabled = []")
}

func TestSkillsList_OnlyInstalled_NoneDisabled(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "coding", "alpha", "Alpha description")
	writeSkill(t, root, "coding", "beta", "Beta description")
	s := newSkillsServer(t, root, nil)

	got := getSkills(t, s)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Name)
	assert.Equal(t, "Alpha description", got[0].Description)
	assert.True(t, got[0].Enabled)
	assert.Equal(t, "beta", got[1].Name)
	assert.True(t, got[1].Enabled)
}

func TestSkillsList_InstalledAndDisabled(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "coding", "alpha", "Alpha description")
	writeSkill(t, root, "coding", "beta", "Beta description")
	s := newSkillsServer(t, root, []string{"beta"})

	got := getSkills(t, s)
	require.Len(t, got, 2)
	assert.True(t, got[0].Enabled, "alpha is enabled")
	assert.False(t, got[1].Enabled, "beta is disabled")
	assert.Equal(t, "Beta description", got[1].Description, "description still populated for disabled skills")
}

func TestSkillsList_GhostsAppearForMissingNames(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "coding", "alpha", "Alpha description")
	s := newSkillsServer(t, root, []string{"phantom"})

	got := getSkills(t, s)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Name)
	assert.True(t, got[0].Enabled)
	assert.Equal(t, "phantom", got[1].Name)
	assert.False(t, got[1].Enabled)
	assert.Empty(t, got[1].Description, "ghost rows have no description")
}

func TestSkillsList_SortedByName(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "coding", "zulu", "")
	writeSkill(t, root, "coding", "alpha", "")
	writeSkill(t, root, "coding", "mike", "")
	s := newSkillsServer(t, root, []string{"yankee"}) // ghost between mike and zulu

	got := getSkills(t, s)
	require.Len(t, got, 4)
	names := []string{got[0].Name, got[1].Name, got[2].Name, got[3].Name}
	assert.Equal(t, []string{"alpha", "mike", "yankee", "zulu"}, names)
}
