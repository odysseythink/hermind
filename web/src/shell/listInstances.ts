import type { AppState } from '../state';

/**
 * listInstanceDirty compares one element of a ShapeList section between
 * state.config and state.originalConfig by index. Reordering the list
 * dirties every row that moved — this is intentional, since preserve is
 * index-based and the UI should surface that.
 */
export function listInstanceDirty(
  state: AppState,
  sectionKey: string,
  index: number,
): boolean {
  const cur = readElement(state.config, sectionKey, index);
  const orig = readElement(state.originalConfig, sectionKey, index);
  if (cur === undefined && orig === undefined) return false;
  if (cur === undefined || orig === undefined) return true;
  return !shallowEqual(cur, orig);
}

function readElement(
  cfg: unknown,
  sectionKey: string,
  index: number,
): Record<string, unknown> | undefined {
  const root = cfg as Record<string, unknown> | null | undefined;
  if (!root) return undefined;
  const sec = root[sectionKey] as Array<Record<string, unknown>> | undefined;
  if (!Array.isArray(sec)) return undefined;
  return sec[index];
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
