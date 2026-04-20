import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { loadExpandedGroups, saveExpandedGroups, STATE_VERSION, STORAGE_KEY } from './persistence';

const DEFAULT_SORTED = ['gateway', 'models', 'observability', 'runtime'];

describe('loadExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => {
    localStorage.clear();
  });

  it('returns the default set on empty storage (gateway + groups with registered sections)', () => {
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });

  it('reads a valid v2 state exactly as stored', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ v: STATE_VERSION, groups: ['models', 'memory'] }),
    );
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.has('memory')).toBe(true);
    expect(got.has('gateway')).toBe(false);
    expect(got.size).toBe(2);
  });

  it('ignores unknown group ids inside a v2 groups array', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ v: STATE_VERSION, groups: ['models', 'not-a-group'] }),
    );
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.size).toBe(1);
  });

  it('falls back to default on malformed JSON', () => {
    localStorage.setItem(STORAGE_KEY, '{not json');
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });

  it('falls back to default when the stored value is not an object', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(true));
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });

  it('falls back to default for objects missing v/groups (Stage 2 never shipped this shape)', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ gateway: true }));
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });

  it('migrates legacy v1 bare-array format to the new default', () => {
    // Pre-Stage-4a users have ['gateway'] or similar stored as a bare array.
    // We reset those users so they see the newly-populated groups expanded.
    localStorage.setItem(STORAGE_KEY, JSON.stringify(['gateway', 'memory', 'skills', 'advanced']));
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });

  it('treats a stored state with the wrong v as legacy and resets', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ v: 999, groups: ['gateway'] }),
    );
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(DEFAULT_SORTED);
  });
});

describe('saveExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('writes a v2 state with sorted groups', () => {
    saveExpandedGroups(new Set(['memory', 'models']));
    expect(localStorage.getItem(STORAGE_KEY)).toBe(
      JSON.stringify({ v: STATE_VERSION, groups: ['memory', 'models'] }),
    );
  });

  it('writes a v2 state with an empty groups array for an empty set', () => {
    saveExpandedGroups(new Set());
    expect(localStorage.getItem(STORAGE_KEY)).toBe(
      JSON.stringify({ v: STATE_VERSION, groups: [] }),
    );
  });

  it('round-trips through load to the same set', () => {
    const input = new Set<'gateway' | 'runtime' | 'models'>(['gateway', 'runtime', 'models']);
    saveExpandedGroups(input);
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway', 'models', 'runtime']);
  });
});
