import type { Config, ConfigSection } from './api/schemas';
import { GROUPS, type GroupId } from './shell/groups';
import { loadExpandedGroups, saveExpandedGroups } from './shell/persistence';
import { setPath } from './util/path';

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
  configSections: ConfigSection[];
  config: Config;
  originalConfig: Config;
  flash: Flash | null;
  shell: ShellSliceState;
  providerModels: Record<string, string[]>;
}

export type Action =
  | { type: 'boot/loaded'; configSections: ConfigSection[]; config: Config }
  | { type: 'boot/failed'; error: string }
  | { type: 'flash'; flash: Flash | null }
  | { type: 'save/start' }
  | { type: 'save/done'; error?: string }
  | { type: 'shell/selectGroup'; group: GroupId | null }
  | { type: 'shell/selectSub'; key: string | null }
  | { type: 'shell/toggleGroup'; group: GroupId }
  | { type: 'edit/config-field'; sectionKey: string; field: string; value: unknown }
  | { type: 'edit/config-scalar'; sectionKey: string; value: unknown }
  | { type: 'edit/keyed-instance-field'; sectionKey: string; subkey?: string; instanceKey: string; field: string; value: unknown }
  | { type: 'keyed-instance/create'; sectionKey: string; subkey?: string; instanceKey: string; initial: Record<string, unknown> }
  | { type: 'keyed-instance/delete'; sectionKey: string; subkey?: string; instanceKey: string }
  | { type: 'edit/list-instance-field'; sectionKey: string; subkey?: string; index: number; field: string; value: unknown }
  | { type: 'list-instance/create'; sectionKey: string; subkey?: string; initial: Record<string, unknown> }
  | { type: 'list-instance/delete'; sectionKey: string; subkey?: string; index: number }
  | { type: 'list-instance/move-up'; sectionKey: string; subkey?: string; index: number }
  | { type: 'list-instance/move-down'; sectionKey: string; subkey?: string; index: number }
  | { type: 'list-instance/move'; sectionKey: string; subkey?: string; from: number; to: number }
  | { type: 'provider/models/loaded'; providerKey: string; models: string[] };

export const initialState: AppState = {
  status: 'booting',
  configSections: [],
  config: {},
  originalConfig: {},
  flash: null,
  shell: {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: loadExpandedGroups(),
  },
  providerModels: {},
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case 'boot/loaded':
      return {
        ...state,
        status: 'ready',
        configSections: action.configSections,
        config: action.config,
        originalConfig: action.config,
      };
    case 'boot/failed':
      return {
        ...state,
        status: 'error',
        flash: { kind: 'err', msg: action.error },
      };
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
    case 'shell/selectGroup':
      return {
        ...state,
        shell: {
          ...state.shell,
          activeGroup: action.group,
          activeSubKey: null,
        },
      };
    case 'shell/selectSub':
      return {
        ...state,
        shell: { ...state.shell, activeSubKey: action.key },
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
          [action.sectionKey]: setPath(prev, action.field, action.value),
        } as typeof state.config,
      };
    }
    case 'edit/config-scalar':
      return {
        ...state,
        config: {
          ...state.config,
          [action.sectionKey]: action.value,
        } as typeof state.config,
      };
    case 'edit/keyed-instance-field': {
      const container =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Record<string, Record<string, unknown>>
          | undefined) ?? {};
      const inst = container[action.instanceKey] ?? {};
      const next = {
        ...container,
        [action.instanceKey]: { ...inst, [action.field]: action.value },
      };
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, next),
      };
    }
    case 'keyed-instance/create': {
      const container =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Record<string, Record<string, unknown>>
          | undefined) ?? {};
      const next = {
        ...container,
        [action.instanceKey]: action.initial,
      };
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, next),
      };
    }
    case 'keyed-instance/delete': {
      const container =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Record<string, Record<string, unknown>>
          | undefined) ?? {};
      if (!(action.instanceKey in container)) {
        return state;
      }
      const next = { ...container };
      delete next[action.instanceKey];
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, next),
      };
    }
    case 'edit/list-instance-field': {
      const list =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Array<Record<string, unknown>>
          | undefined) ?? [];
      if (action.index < 0 || action.index >= list.length) {
        return state;
      }
      const nextList = list.slice();
      nextList[action.index] = { ...nextList[action.index], [action.field]: action.value };
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, nextList),
      };
    }
    case 'list-instance/create': {
      const list =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Array<Record<string, unknown>>
          | undefined) ?? [];
      const nextList = list.concat([{ ...action.initial }]);
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, nextList),
      };
    }
    case 'list-instance/delete': {
      const list =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Array<Record<string, unknown>>
          | undefined) ?? [];
      if (action.index < 0 || action.index >= list.length) {
        return state;
      }
      const nextList = list.slice();
      nextList.splice(action.index, 1);
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, nextList),
      };
    }
    case 'list-instance/move-up':
    case 'list-instance/move-down': {
      const list =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Array<Record<string, unknown>>
          | undefined) ?? [];
      const target = action.type === 'list-instance/move-up' ? action.index - 1 : action.index + 1;
      if (action.index < 0 || action.index >= list.length) return state;
      if (target < 0 || target >= list.length) return state;
      const nextList = list.slice();
      [nextList[action.index], nextList[target]] = [nextList[target], nextList[action.index]];
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, nextList),
      };
    }
    case 'list-instance/move': {
      const list =
        (resolveContainer(state.config, action.sectionKey, action.subkey) as
          | Array<Record<string, unknown>>
          | undefined) ?? [];
      if (action.from === action.to) return state;
      if (action.from < 0 || action.from >= list.length) return state;
      if (action.to < 0 || action.to >= list.length) return state;
      const nextList = list.slice();
      const [moved] = nextList.splice(action.from, 1);
      nextList.splice(action.to, 0, moved);
      return {
        ...state,
        config: writeContainer(state.config, action.sectionKey, action.subkey, nextList),
      };
    }
    case 'provider/models/loaded':
      return {
        ...state,
        providerModels: {
          ...state.providerModels,
          [action.providerKey]: action.models,
        },
      };
  }
}

function resolveContainer(
  config: Config,
  sectionKey: string,
  subkey: string | undefined,
): unknown {
  const cfg = config as unknown as Record<string, unknown>;
  const top = cfg[sectionKey];
  if (top === undefined || top === null) return undefined;
  if (!subkey) return top;
  if (typeof top !== 'object' || Array.isArray(top)) return undefined;
  return (top as Record<string, unknown>)[subkey];
}

function writeContainer(
  config: Config,
  sectionKey: string,
  subkey: string | undefined,
  next: unknown,
): Config {
  if (!subkey) {
    return { ...config, [sectionKey]: next } as typeof config;
  }
  const cfg = config as unknown as Record<string, unknown>;
  const prev = (cfg[sectionKey] as Record<string, unknown> | undefined) ?? {};
  return {
    ...config,
    [sectionKey]: { ...prev, [subkey]: next },
  } as typeof config;
}

export function groupDirty(state: AppState, group: GroupId): boolean {
  const def = GROUPS.find((g) => g.id === group);
  if (!def) return false;
  const a = state.config as unknown as Record<string, unknown>;
  const b = state.originalConfig as unknown as Record<string, unknown>;
  for (const k of def.configKeys) {
    if (!deepEqual(a[k], b[k])) return true;
  }
  return false;
}

export function dirtyGroups(state: AppState): Set<GroupId> {
  const out = new Set<GroupId>();
  for (const g of GROUPS) {
    if (groupDirty(state, g.id)) out.add(g.id);
  }
  return out;
}

export function totalDirtyCount(state: AppState): number {
  let n = 0;
  for (const g of GROUPS) {
    if (groupDirty(state, g.id)) n++;
  }
  return n;
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
