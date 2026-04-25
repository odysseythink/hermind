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

  it('registers Stage 4a sections: model under models, auxiliary under runtime', () => {
    const model = findSection('model');
    expect(model, 'missing model').toBeDefined();
    expect(model!.groupId).toBe('models');
    expect(model!.plannedStage).toBe('done');

    const aux = findSection('auxiliary');
    expect(aux, 'missing auxiliary').toBeDefined();
    expect(aux!.groupId).toBe('runtime');
    expect(aux!.plannedStage).toBe('done');
  });

  it('runtime group exposes storage, agent, auxiliary, terminal in declaration order', () => {
    const runtime = sectionsInGroup('runtime');
    expect(runtime.map(s => s.key)).toEqual(['storage', 'agent', 'auxiliary', 'terminal']);
  });

  it('observability group exposes logging, metrics, tracing in declaration order', () => {
    const observability = sectionsInGroup('observability');
    expect(observability.map(s => s.key)).toEqual(['logging', 'metrics', 'tracing']);
  });

  it('registers Stage 4b section: providers under models', () => {
    const p = findSection('providers');
    expect(p, 'missing providers').toBeDefined();
    expect(p!.groupId).toBe('models');
    expect(p!.plannedStage).toBe('done');
  });

  it('registers Stage 4c section: fallback_providers under models', () => {
    const fb = findSection('fallback_providers');
    expect(fb, 'missing fallback_providers').toBeDefined();
    expect(fb!.groupId).toBe('models');
    expect(fb!.plannedStage).toBe('done');
  });

  it('models group exposes model, providers, fallback_providers in declaration order', () => {
    const models = sectionsInGroup('models');
    expect(models.map(s => s.key)).toEqual(['model', 'providers', 'fallback_providers']);
  });

  it('memory group exposes presence in declaration order', () => {
    const memory = sectionsInGroup('memory');
    expect(memory.map(s => s.key)).toEqual(['presence']);
  });

  it('advanced group exposes proxy, benchmark in declaration order', () => {
    const advanced = sectionsInGroup('advanced');
    expect(advanced.map(s => s.key)).toEqual(['proxy', 'benchmark']);
  });

  it('sectionsInGroup returns [] for a group with no registered sections', () => {
    expect(sectionsInGroup('skills')).toEqual([]);
    expect(sectionsInGroup('gateway')).toEqual([]);
  });
});
