import { GROUP_IDS, type GroupId } from './groups';

export const STORAGE_KEY = 'hermind.shell.expandedGroups';
const DEFAULT_EXPANDED: readonly GroupId[] = ['gateway'];

export function loadExpandedGroups(): Set<GroupId> {
  const raw = tryRead();
  if (raw === null) return new Set(DEFAULT_EXPANDED);
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return new Set(DEFAULT_EXPANDED);
  }
  if (!Array.isArray(parsed)) return new Set(DEFAULT_EXPANDED);
  const out = new Set<GroupId>();
  for (const v of parsed) {
    if (typeof v === 'string' && GROUP_IDS.has(v as GroupId)) {
      out.add(v as GroupId);
    }
  }
  return out;
}

export function saveExpandedGroups(set: Set<GroupId>): void {
  const arr = Array.from(set).sort();
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(arr));
  } catch {
    // quota exceeded or storage disabled — silently drop
  }
}

function tryRead(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}
