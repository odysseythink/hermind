import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { loadExpandedGroups, saveExpandedGroups, STORAGE_KEY } from './persistence';

describe('loadExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => {
    localStorage.clear();
  });

  it('returns the default set {gateway} when localStorage is empty', () => {
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });

  it('reads a valid persisted array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(['models', 'memory']));
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.has('memory')).toBe(true);
    expect(got.has('gateway')).toBe(false);
  });

  it('ignores unknown group ids in the stored array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(['models', 'not-a-group']));
    const got = loadExpandedGroups();
    expect(got.has('models')).toBe(true);
    expect(got.size).toBe(1);
  });

  it('falls back to default on malformed JSON', () => {
    localStorage.setItem(STORAGE_KEY, '{not json');
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });

  it('falls back to default when the stored value is not an array', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ gateway: true }));
    const got = loadExpandedGroups();
    expect(Array.from(got).sort()).toEqual(['gateway']);
  });
});

describe('saveExpandedGroups', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('writes a sorted JSON array of group ids', () => {
    saveExpandedGroups(new Set(['memory', 'models']));
    expect(localStorage.getItem(STORAGE_KEY)).toBe(JSON.stringify(['memory', 'models']));
  });

  it('writes an empty array for an empty set', () => {
    saveExpandedGroups(new Set());
    expect(localStorage.getItem(STORAGE_KEY)).toBe('[]');
  });
});
