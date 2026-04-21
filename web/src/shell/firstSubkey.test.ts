import { describe, it, expect } from 'vitest';
import { firstSubkeyForGroup } from './firstSubkey';
import type { ConfigSection } from '../api/schemas';

const sections: ConfigSection[] = [
  { key: 'model',              group_id: 'models',        label: 'M', shape: 'scalar', fields: [] },
  { key: 'providers',          group_id: 'models',        label: 'P', shape: 'keyed_map', fields: [] },
  { key: 'fallback_providers', group_id: 'models',        label: 'F', shape: 'list', fields: [] },
  { key: 'memory',             group_id: 'memory',        label: 'Mem', shape: 'map', fields: [] },
  { key: 'skills',             group_id: 'skills',        label: 'S', shape: 'map', fields: [] },
  { key: 'storage',            group_id: 'runtime',       label: 'St', fields: [] },
  { key: 'agent',              group_id: 'runtime',       label: 'A', fields: [] },
  { key: 'logging',            group_id: 'observability', label: 'L', fields: [] },
  { key: 'mcp',                group_id: 'advanced',      label: 'MCP', shape: 'keyed_map', fields: [] },
  { key: 'browser',            group_id: 'advanced',      label: 'B', shape: 'map', fields: [] },
  { key: 'cron',               group_id: 'advanced',      label: 'C', shape: 'list', fields: [] },
];

describe('firstSubkeyForGroup', () => {
  it('returns null for gateway (dedicated panel)', () => {
    expect(firstSubkeyForGroup('gateway', sections, [])).toBeNull();
  });

  it('returns the first provider instance for models when any exist', () => {
    expect(firstSubkeyForGroup('models', sections, ['anthropic_main', 'openai'])).toBe('anthropic_main');
  });

  it('falls back to the scalar "model" section when no providers exist', () => {
    expect(firstSubkeyForGroup('models', sections, [])).toBe('model');
  });

  it('picks the first map section for memory', () => {
    expect(firstSubkeyForGroup('memory', sections, [])).toBe('memory');
  });

  it('picks the first map section for skills', () => {
    expect(firstSubkeyForGroup('skills', sections, [])).toBe('skills');
  });

  it('picks the first section for runtime (default shape = map)', () => {
    expect(firstSubkeyForGroup('runtime', sections, [])).toBe('storage');
  });

  it('picks the first section for observability', () => {
    expect(firstSubkeyForGroup('observability', sections, [])).toBe('logging');
  });

  it('skips keyed_map/list shapes for advanced and picks the first map section', () => {
    expect(firstSubkeyForGroup('advanced', sections, [])).toBe('browser');
  });

  it('returns null when the group has no scalar/map sections', () => {
    const onlyLists: ConfigSection[] = [
      { key: 'mcp',  group_id: 'advanced', label: 'MCP', shape: 'keyed_map', fields: [] },
      { key: 'cron', group_id: 'advanced', label: 'C',   shape: 'list',      fields: [] },
    ];
    expect(firstSubkeyForGroup('advanced', onlyLists, [])).toBeNull();
  });
});
