import type { AppState } from '../state';

/**
 * Returns true when a ShapeKeyedMap instance differs between the current
 * and original config slices. Used by ModelsSidebar to render a per-instance
 * dirty dot. An instance is dirty if it is present-only-in-one-side or if
 * any field value diverges.
 */
export function keyedInstanceDirty(
  state: AppState,
  sectionKey: string,
  instanceKey: string,
): boolean {
  const cur = readInstance(state.config, sectionKey, instanceKey);
  const orig = readInstance(state.originalConfig, sectionKey, instanceKey);
  if (cur === undefined && orig === undefined) return false;
  if (cur === undefined || orig === undefined) return true;
  return !shallowEqual(cur, orig);
}

function readInstance(
  cfg: unknown,
  sectionKey: string,
  instanceKey: string,
): Record<string, unknown> | undefined {
  const root = cfg as Record<string, unknown> | null | undefined;
  if (!root) return undefined;
  const sec = root[sectionKey] as Record<string, unknown> | undefined;
  if (!sec) return undefined;
  return sec[instanceKey] as Record<string, unknown> | undefined;
}

function shallowEqual(a: Record<string, unknown>, b: Record<string, unknown>): boolean {
  const ak = Object.keys(a);
  const bk = Object.keys(b);
  if (ak.length !== bk.length) return false;
  for (const k of ak) {
    if (a[k] !== b[k]) return false;
  }
  return true;
}
