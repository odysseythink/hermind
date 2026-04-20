// getPath walks a dotted path down an object and returns the leaf value.
// Returns undefined if any intermediate key is missing or the root is not
// an object. Flat (no-dot) paths behave exactly like obj[path].
export function getPath(obj: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>(
    (o, k) => (o as Record<string, unknown> | undefined)?.[k],
    obj,
  );
}

// setPath returns a new object with value written at path. Intermediate
// objects are created as empty {} when missing. The input is never
// mutated; flat paths produce {...obj, [head]: value}.
export function setPath(
  obj: Record<string, unknown>,
  path: string,
  value: unknown,
): Record<string, unknown> {
  const [head, ...rest] = path.split('.');
  if (rest.length === 0) return { ...obj, [head]: value };
  const inner = (obj[head] as Record<string, unknown> | undefined) ?? {};
  return { ...obj, [head]: setPath(inner, rest.join('.'), value) };
}
