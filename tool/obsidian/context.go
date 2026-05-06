package obsidian

// VaultPathKey is the context key used to pass the Obsidian vault path
// from the HTTP handler down to the tool handlers.
type vaultPathKey struct{}

var VaultPathKey = vaultPathKey{}
