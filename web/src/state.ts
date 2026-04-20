import type { Config, PlatformInstance, SchemaDescriptor } from './api/schemas';
import { GROUPS, type GroupId } from './shell/groups';
import { loadExpandedGroups, saveExpandedGroups } from './shell/persistence';

export type Status = 'booting' | 'ready' | 'saving' | 'applying' | 'error';

export interface Flash {
  kind: 'ok' | 'err';
  msg: string;
}

export interface ShellSliceState {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
}

export interface AppState {
  status: Status;
  descriptors: SchemaDescriptor[];
  config: Config;
  originalConfig: Config;
  /** Legacy field retained for existing IM code paths.
   *  Mirrors shell.activeSubKey when shell.activeGroup === 'gateway'. */
  selectedKey: string | null;
  flash: Flash | null;
  shell: ShellSliceState;
}

export type Action =
  | { type: 'boot/loaded'; descriptors: SchemaDescriptor[]; config: Config }
  | { type: 'boot/failed'; error: string }
  | { type: 'select'; key: string | null }
  | { type: 'flash'; flash: Flash | null }
  | { type: 'save/start' }
  | { type: 'save/done'; error?: string }
  | { type: 'apply/start' }
  | { type: 'apply/done'; error?: string }
  | { type: 'edit/field'; key: string; field: string; value: string }
  | { type: 'edit/enabled'; key: string; enabled: boolean }
  | { type: 'instance/delete'; key: string }
  | { type: 'instance/create'; key: string; platformType: string }
  | { type: 'shell/selectGroup'; group: GroupId | null }
  | { type: 'shell/selectSub'; key: string | null }
  | { type: 'shell/toggleGroup'; group: GroupId }
  | { type: 'edit/config-field'; sectionKey: string; field: string; value: unknown };

export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
  shell: {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: loadExpandedGroups(),
  },
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case 'boot/loaded':
      return {
        ...state,
        status: 'ready',
        descriptors: action.descriptors,
        config: action.config,
        originalConfig: action.config,
      };
    case 'boot/failed':
      return {
        ...state,
        status: 'error',
        flash: { kind: 'err', msg: action.error },
      };
    case 'select':
      return { ...state, selectedKey: action.key };
    case 'flash':
      return { ...state, flash: action.flash };
    case 'save/start':
      return { ...state, status: 'saving', flash: null };
    case 'save/done':
      return action.error
        ? { ...state, status: 'ready', flash: { kind: 'err', msg: action.error } }
        : {
            ...state,
            status: 'ready',
            originalConfig: state.config,
            flash: { kind: 'ok', msg: 'Saved.' },
          };
    case 'apply/start':
      return { ...state, status: 'applying', flash: null };
    case 'apply/done':
      return action.error
        ? { ...state, status: 'ready', flash: { kind: 'err', msg: action.error } }
        : { ...state, status: 'ready', flash: { kind: 'ok', msg: 'Applied.' } };
    case 'edit/field':
      return { ...state, config: setField(state.config, action.key, action.field, action.value) };
    case 'edit/enabled':
      return { ...state, config: setEnabled(state.config, action.key, action.enabled) };
    case 'instance/delete':
      return {
        ...state,
        config: deleteInstance(state.config, action.key),
        selectedKey: state.selectedKey === action.key ? null : state.selectedKey,
      };
    case 'instance/create': {
      const plats = { ...(state.config.gateway?.platforms ?? {}) };
      plats[action.key] = {
        enabled: true,
        type: action.platformType,
        options: {},
      };
      return {
        ...state,
        config: { ...state.config, gateway: { ...(state.config.gateway ?? {}), platforms: plats } },
        selectedKey: action.key,
      };
    }
    case 'shell/selectGroup': {
      const next = {
        ...state,
        shell: {
          ...state.shell,
          activeGroup: action.group,
          activeSubKey: null,
        },
      };
      // Keep legacy selectedKey in sync so the existing IM Editor path keeps working
      return { ...next, selectedKey: null };
    }
    case 'shell/selectSub':
      return {
        ...state,
        shell: { ...state.shell, activeSubKey: action.key },
        selectedKey:
          state.shell.activeGroup === 'gateway' ? action.key : state.selectedKey,
      };
    case 'shell/toggleGroup': {
      const expanded = new Set(state.shell.expandedGroups);
      if (expanded.has(action.group)) expanded.delete(action.group);
      else expanded.add(action.group);
      saveExpandedGroups(expanded);
      return { ...state, shell: { ...state.shell, expandedGroups: expanded } };
    }
    case 'edit/config-field': {
      const cfg = state.config as unknown as Record<string, unknown>;
      const prev = (cfg[action.sectionKey] as Record<string, unknown> | undefined) ?? {};
      return {
        ...state,
        config: {
          ...state.config,
          [action.sectionKey]: { ...prev, [action.field]: action.value },
        } as typeof state.config,
      };
    }
  }
}

/** listInstances returns keys in the current config.gateway.platforms map, sorted. */
export function listInstances(state: AppState): string[] {
  const plats = state.config.gateway?.platforms ?? {};
  return Object.keys(plats).sort();
}

/** dirtyCount returns how many instance keys differ between config and
 * originalConfig. Added keys count. Deleted keys count. Any mutation
 * inside a surviving key counts as one. */
export function dirtyCount(state: AppState): number {
  const a = state.config.gateway?.platforms ?? {};
  const b = state.originalConfig.gateway?.platforms ?? {};
  const keys = new Set<string>([...Object.keys(a), ...Object.keys(b)]);
  let n = 0;
  for (const k of keys) {
    if (!shallowEqualInstance(a[k], b[k])) n++;
  }
  return n;
}

/** instanceDirty returns true when a single key differs between the
 * current config and the snapshot. Used by the Sidebar to render a
 * per-instance unsaved indicator. */
export function instanceDirty(state: AppState, key: string): boolean {
  const a = state.config.gateway?.platforms?.[key];
  const b = state.originalConfig.gateway?.platforms?.[key];
  return !shallowEqualInstance(a, b);
}

function shallowEqualInstance(
  a: PlatformInstance | undefined,
  b: PlatformInstance | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  if (a.type !== b.type) return false;
  if ((a.enabled ?? false) !== (b.enabled ?? false)) return false;
  const ao = a.options ?? {};
  const bo = b.options ?? {};
  const keys = new Set<string>([...Object.keys(ao), ...Object.keys(bo)]);
  for (const k of keys) {
    if ((ao[k] ?? '') !== (bo[k] ?? '')) return false;
  }
  return true;
}

function setField(config: Config, key: string, field: string, value: string): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  const prev = plats[key];
  if (!prev) return config;
  const opts = { ...(prev.options ?? {}), [field]: value };
  plats[key] = { ...prev, options: opts };
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}

function setEnabled(config: Config, key: string, enabled: boolean): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  const prev = plats[key];
  if (!prev) return config;
  plats[key] = { ...prev, enabled };
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}

function deleteInstance(config: Config, key: string): Config {
  const plats = { ...(config.gateway?.platforms ?? {}) };
  if (!(key in plats)) return config;
  delete plats[key];
  return { ...config, gateway: { ...(config.gateway ?? {}), platforms: plats } };
}

/** groupDirty returns true if the config slice for the group differs from
 *  the originalConfig snapshot. Stage 1: only 'gateway' can be dirty. */
export function groupDirty(state: AppState, group: GroupId): boolean {
  if (group === 'gateway') {
    return dirtyCount(state) > 0;
  }
  // For non-gateway groups, compare the relevant configKeys shallowly.
  const def = GROUPS.find(g => g.id === group);
  if (!def) return false;
  const a = state.config as unknown as Record<string, unknown>;
  const b = state.originalConfig as unknown as Record<string, unknown>;
  for (const k of def.configKeys) {
    if (!deepEqual(a[k], b[k])) return true;
  }
  return false;
}

/** dirtyGroups returns the set of groups with unsaved changes. */
export function dirtyGroups(state: AppState): Set<GroupId> {
  const out = new Set<GroupId>();
  for (const g of GROUPS) {
    if (groupDirty(state, g.id)) out.add(g.id);
  }
  return out;
}

/** totalDirtyCount returns the total number of dirty sub-items across all
 *  groups. Stage 1: equals dirtyCount (the IM instance diff count); later
 *  stages may sum per-group sub-counts. */
export function totalDirtyCount(state: AppState): number {
  return dirtyCount(state);
}

function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) return false;
  if (typeof a !== 'object') return false;
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b)) return false;
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!deepEqual(a[i], b[i])) return false;
    }
    return true;
  }
  const ao = a as Record<string, unknown>;
  const bo = b as Record<string, unknown>;
  const keys = new Set<string>([...Object.keys(ao), ...Object.keys(bo)]);
  for (const k of keys) {
    if (!deepEqual(ao[k], bo[k])) return false;
  }
  return true;
}
