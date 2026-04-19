import type { Config, PlatformInstance, SchemaDescriptor } from './api/schemas';

export type Status = 'booting' | 'ready' | 'saving' | 'applying' | 'error';

export interface Flash {
  kind: 'ok' | 'err';
  msg: string;
}

export interface AppState {
  status: Status;
  descriptors: SchemaDescriptor[];
  config: Config;
  originalConfig: Config;
  selectedKey: string | null;
  flash: Flash | null;
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
  | { type: 'instance/create'; key: string; platformType: string };

export const initialState: AppState = {
  status: 'booting',
  descriptors: [],
  config: {},
  originalConfig: {},
  selectedKey: null,
  flash: null,
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
