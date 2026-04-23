// Hardcoded model list until a /api/models discovery endpoint exists.
// Empty string means "use session default / server fallback".
export const MODEL_OPTIONS: readonly string[] = [
  '',
  'claude-opus-4-7',
  'claude-sonnet-4-6',
  'gpt-4',
];
