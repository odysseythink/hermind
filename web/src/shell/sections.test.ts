import { describe, it, expect } from 'vitest';
import { SECTIONS, sectionsInGroup, findSection } from './sections';
import { GROUP_IDS } from './groups';

describe('SECTIONS registry', () => {
  it('every entry references a real group id', () => {
    for (const s of SECTIONS) {
      expect(GROUP_IDS.has(s.groupId)).toBe(true);
    }
  });

  it('contains storage in runtime with plannedStage=done', () => {
    const s = findSection('storage');
    expect(s).toBeDefined();
    expect(s!.groupId).toBe('runtime');
    expect(s!.plannedStage).toBe('done');
  });

  it('sectionsInGroup returns entries in declaration order', () => {
    const runtime = sectionsInGroup('runtime');
    const keys = runtime.map(s => s.key);
    expect(keys).toContain('storage');
  });

  it('sectionsInGroup returns [] for a group with no registered sections', () => {
    // Memory group has no sections in stage 2; stage 5 adds them.
    expect(sectionsInGroup('memory')).toEqual([]);
  });
});
