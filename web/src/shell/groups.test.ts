import { describe, it, expect } from 'vitest';
import { GROUP_IDS, GROUPS, findGroup, type GroupId } from './groups';

describe('GROUPS table', () => {
  it('contains exactly 7 groups', () => {
    expect(GROUPS).toHaveLength(7);
  });

  it('has the expected ids in fixed display order', () => {
    const ids: GroupId[] = GROUPS.map(g => g.id);
    expect(ids).toEqual([
      'models',
      'gateway',
      'memory',
      'skills',
      'runtime',
      'advanced',
      'observability',
    ]);
  });

  it('has no duplicate ids', () => {
    const ids = GROUPS.map(g => g.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('every group has label, plannedStage, configKeys, description, bullets', () => {
    for (const g of GROUPS) {
      expect(g.label).toBeTruthy();
      expect(g.plannedStage).toBeTruthy();
      expect(Array.isArray(g.configKeys)).toBe(true);
      expect(g.configKeys.length).toBeGreaterThan(0);
      expect(g.description).toBeTruthy();
      expect(Array.isArray(g.bullets)).toBe(true);
      expect(g.bullets.length).toBeGreaterThan(0);
    }
  });

  it('GROUP_IDS set matches GROUPS entries', () => {
    expect(Array.from(GROUP_IDS).sort()).toEqual(GROUPS.map(g => g.id).sort());
  });
});

describe('findGroup', () => {
  it('returns the matching group for a known id', () => {
    expect(findGroup('gateway').label).toBe('Gateway');
  });

  it('throws for an unknown id', () => {
    expect(() => findGroup('bogus' as GroupId)).toThrow();
  });
});
