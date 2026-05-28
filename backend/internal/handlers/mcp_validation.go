package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

// validateArgsAgainstSchema returns nil if args satisfies inputSchema, or an
// error whose Error() contains the aggregated mismatch messages. A nil or
// empty schema returns nil unconditionally.
func validateArgsAgainstSchema(args map[string]any, inputSchema json.RawMessage) error {
	if len(inputSchema) == 0 {
		return nil
	}
	schemaLoader := gojsonschema.NewBytesLoader(inputSchema)
	argsBytes, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}
	docLoader := gojsonschema.NewBytesLoader(argsBytes)
	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("schema validate: %w", err)
	}
	if result.Valid() {
		return nil
	}
	msgs := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		msgs = append(msgs, e.String())
	}
	return fmt.Errorf("schema validation failed: %s", strings.Join(msgs, "; "))
}

const (
	minTimeout = 1 * time.Second
	maxTimeout = 300 * time.Second
)

// parseTimeoutParam parses a ?timeout=<duration> query value. An empty string
// returns (0, nil) — callers substitute their default. Out-of-range or
// malformed values return an error.
func parseTimeoutParam(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", s, err)
	}
	if d < minTimeout || d > maxTimeout {
		return 0, fmt.Errorf("timeout %s out of range [%s, %s]", d, minTimeout, maxTimeout)
	}
	return d, nil
}
