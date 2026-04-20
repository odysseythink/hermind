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

  it('registers all five Stage 3 sections as done', () => {
    const stage3 = {
      logging: 'observability',
      metrics: 'observability',
      tracing: 'observability',
      agent: 'runtime',
      terminal: 'runtime',
    };
    for (const [key, group] of Object.entries(stage3)) {
      const s = findSection(key);
      expect(s, `missing ${key}`).toBeDefined();
      expect(s!.groupId).toBe(group);
      expect(s!.plannedStage).toBe('done');
    }
  });

  it('runtime group exposes storage, agent, terminal in declaration order', () => {
    const runtime = sectionsInGroup('runtime');
    expect(runtime.map(s => s.key)).toEqual(['storage', 'agent', 'terminal']);
  });

  it('observability group exposes logging, metrics, tracing in declaration order', () => {
    const observability = sectionsInGroup('observability');
    expect(observability.map(s => s.key)).toEqual(['logging', 'metrics', 'tracing']);
  });

  it('sectionsInGroup returns [] for a group with no registered sections', () => {
    expect(sectionsInGroup('memory')).toEqual([]);
  });
});
