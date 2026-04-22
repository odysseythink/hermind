package api

// Size caps enforced by PATCH /api/sessions/{id}. Values chosen to be
// generous for normal use while preventing accidental or malicious
// blobs from entering the sessions table.
const (
	MaxSessionTitleBytes = 256       // bytes, not runes — conservative
	MaxSystemPromptBytes = 32 * 1024 // 32 KB
	MaxModelNameBytes    = 128
)
