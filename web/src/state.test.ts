import { describe, expect, it } from 'vitest';
import type { Config, PlatformInstance, SchemaDescriptor } from './api/schemas';
import {
  dirtyCount,
  initialState,
  instanceDirty,
  listInstances,
  reducer,
} from './state';

function cfg(plats: Record<string, PlatformInstance>): Config {
  return { gateway: { platforms: plats } };
}

const emptyDescriptors: SchemaDescriptor[] = [];

describe('reducer — boot/loaded', () => {
  it('transitions to ready and seeds originalConfig', () => {
    const state = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ x: { type: 't', enabled: true } }),
    });
    expect(state.status).toBe('ready');
    expect(state.config).toEqual(state.originalConfig);
  });
});

describe('reducer — edit/field', () => {
  it('mutates options without touching the key map shape', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 'telegram', enabled: true, options: { token: 'old' } } }),
    });
    const s1 = reducer(s0, { type: 'edit/field', key: 'k', field: 'token', value: 'new' });
    expect(s1.config.gateway?.platforms?.k?.options?.token).toBe('new');
    expect(s1.originalConfig.gateway?.platforms?.k?.options?.token).toBe('old');
  });

  it('is a no-op for unknown keys', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true, options: {} } }),
    });
    const s1 = reducer(s0, { type: 'edit/field', key: 'nope', field: 'x', value: 'v' });
    expect(s1.config).toEqual(s0.config);
  });
});

describe('reducer — edit/enabled', () => {
  it('flips the enabled flag', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true } }),
    });
    const s1 = reducer(s0, { type: 'edit/enabled', key: 'k', enabled: false });
    expect(s1.config.gateway?.platforms?.k?.enabled).toBe(false);
  });
});

describe('reducer — instance/delete', () => {
  it('removes the key and clears selectedKey when it pointed to it', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true } }),
    });
    const s1 = reducer(s0, { type: 'select', key: 'k' });
    const s2 = reducer(s1, { type: 'instance/delete', key: 'k' });
    expect(s2.config.gateway?.platforms?.k).toBeUndefined();
    expect(s2.selectedKey).toBeNull();
  });

  it('preserves selectedKey when deleting a different key', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({
        a: { type: 't', enabled: true },
        b: { type: 't', enabled: true },
      }),
    });
    const s1 = reducer(s0, { type: 'select', key: 'a' });
    const s2 = reducer(s1, { type: 'instance/delete', key: 'b' });
    expect(s2.selectedKey).toBe('a');
  });
});

describe('reducer — instance/create', () => {
  it('seeds an enabled empty-options entry and selects it', () => {
    const s1 = reducer(initialState, {
      type: 'instance/create',
      key: 'new_key',
      platformType: 'telegram',
    });
    const inst = s1.config.gateway?.platforms?.new_key;
    expect(inst).toEqual({ enabled: true, type: 'telegram', options: {} });
    expect(s1.selectedKey).toBe('new_key');
  });
});

describe('reducer — save/done', () => {
  it('syncs originalConfig on success', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true, options: { x: 'old' } } }),
    });
    const s1 = reducer(s0, { type: 'edit/field', key: 'k', field: 'x', value: 'new' });
    expect(instanceDirty(s1, 'k')).toBe(true);
    const s2 = reducer(s1, { type: 'save/done' });
    expect(s2.status).toBe('ready');
    expect(instanceDirty(s2, 'k')).toBe(false);
  });

  it('leaves originalConfig alone on error', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true } }),
    });
    const s1 = reducer(s0, { type: 'edit/enabled', key: 'k', enabled: false });
    const s2 = reducer(s1, { type: 'save/done', error: 'HTTP 500' });
    expect(s2.originalConfig.gateway?.platforms?.k?.enabled).toBe(true);
    expect(s2.flash?.kind).toBe('err');
  });
});

describe('selectors', () => {
  it('listInstances returns sorted keys', () => {
    const s = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({
        b: { type: 't', enabled: true },
        a: { type: 't', enabled: true },
      }),
    });
    expect(listInstances(s)).toEqual(['a', 'b']);
  });

  it('dirtyCount counts per-key differences', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({
        a: { type: 't', enabled: true },
        b: { type: 't', enabled: true },
      }),
    });
    expect(dirtyCount(s0)).toBe(0);
    const s1 = reducer(s0, { type: 'edit/enabled', key: 'a', enabled: false });
    expect(dirtyCount(s1)).toBe(1);
    const s2 = reducer(s1, { type: 'instance/create', key: 'c', platformType: 't' });
    expect(dirtyCount(s2)).toBe(2);
  });

  it('instanceDirty normalizes absent options key with empty string', () => {
    const s0 = reducer(initialState, {
      type: 'boot/loaded',
      descriptors: emptyDescriptors,
      config: cfg({ k: { type: 't', enabled: true, options: {} } }),
    });
    const s1 = reducer(s0, { type: 'edit/field', key: 'k', field: 'x', value: '' });
    expect(instanceDirty(s1, 'k')).toBe(false);
    const s2 = reducer(s1, { type: 'edit/field', key: 'k', field: 'x', value: 'y' });
    expect(instanceDirty(s2, 'k')).toBe(true);
  });
});
