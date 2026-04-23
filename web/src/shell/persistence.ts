import { GROUP_IDS, type GroupId } from './groups';

export const STORAGE_KEY = 'hermind.shell.expandedGroups';

// Storage schema version. Bump whenever the default-expanded set changes
// (typically when new groups gain their first registered sections) so
// existing users see those groups expanded on first load after upgrade
// instead of remaining silently hidden behind a stale persisted state.
//
//   v1 — pre-Stage-3; stored as bare string array; defaulted to ['gateway']
//   v2 — Stage 4a+; stored as { v, groups }; default also expands runtime,
//        observability, and models (every group with registered sections).
export const STATE_VERSION = 2;

// Default expansion on first load or after a version bump. Includes every
// group that currently has registered descriptors plus gateway (always on).
const DEFAULT_EXPANDED: readonly GroupId[] = ['runtime', 'observability', 'models'];

interface StoredState {
  v: number;
  groups: string[];
}

export function loadExpandedGroups(): Set<GroupId> {
  const raw = tryRead();
  if (raw === null) return new Set(DEFAULT_EXPANDED);
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return new Set(DEFAULT_EXPANDED);
  }
  // Reject legacy bare-array format (v1) and any version mismatch.
  if (!isStoredState(parsed) || parsed.v !== STATE_VERSION) {
    return new Set(DEFAULT_EXPANDED);
  }
  const out = new Set<GroupId>();
  for (const v of parsed.groups) {
    if (typeof v === 'string' && GROUP_IDS.has(v as GroupId)) {
      out.add(v as GroupId);
    }
  }
  return out;
}

export function saveExpandedGroups(set: Set<GroupId>): void {
  const state: StoredState = { v: STATE_VERSION, groups: Array.from(set).sort() };
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
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

function isStoredState(v: unknown): v is StoredState {
  if (typeof v !== 'object' || v === null) return false;
  const o = v as Partial<StoredState>;
  return typeof o.v === 'number' && Array.isArray(o.groups);
}
