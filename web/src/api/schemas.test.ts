import { describe, expect, it } from 'vitest';
import {
  ConfigFieldSchema,
  ConfigResponseSchema,
  ConfigSchemaResponseSchema,
  ConfigSectionSchema,
  ConversationHistoryResponseSchema,
  MetaResponseSchema,
  ProviderModelsResponseSchema,
  StoredMessageSchema,
} from './schemas';

describe('ConfigResponseSchema', () => {
  it('accepts an unknown top-level key (catchall unknown)', () => {
    const r = ConfigResponseSchema.parse({
      config: {
        model: 'claude-sonnet-4-5',
        providers: { anthropic: { api_key: 'redacted' } },
      },
    });
    expect((r.config as Record<string, unknown>).model).toBe('claude-sonnet-4-5');
  });
});

describe('ConfigSchemaResponseSchema', () => {
  it('accepts a storage section with visible_when', () => {
    const good = {
      sections: [
        {
          key: 'storage',
          label: 'Storage',
          group_id: 'runtime',
          fields: [
            { name: 'driver', label: 'Driver', kind: 'enum',
              required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
            { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
              visible_when: { field: 'driver', equals: 'sqlite' } },
          ],
        },
      ],
    };
    expect(() => ConfigSchemaResponseSchema.parse(good)).not.toThrow();
  });

  it('rejects a response missing sections', () => {
    expect(() => ConfigSchemaResponseSchema.parse({})).toThrow();
  });

  it('rejects a field with unknown kind', () => {
    const bad = {
      sections: [
        { key: 's', label: 'S', group_id: 'runtime',
          fields: [{ name: 'x', label: 'X', kind: 'mystery' }] },
      ],
    };
    expect(() => ConfigSchemaResponseSchema.parse(bad)).toThrow();
  });
});

describe('ConfigSectionSchema — shape discriminant', () => {
  it('accepts sections without a shape key (defaults to map)', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'storage',
      label: 'Storage',
      group_id: 'runtime',
      fields: [{ name: 'driver', label: 'Driver', kind: 'enum', enum: ['sqlite'] }],
    });
    expect(parsed.shape).toBeUndefined();
  });

  it('accepts shape: "scalar"', () => {
    const parsed = ConfigSectionSchema.parse({
      key: 'model',
      label: 'Default model',
      group_id: 'models',
      shape: 'scalar',
      fields: [{ name: 'model', label: 'Model', kind: 'string' }],
    });
    expect(parsed.shape).toBe('scalar');
  });

  it('rejects unknown shape values', () => {
    expect(() =>
      ConfigSectionSchema.parse({
        key: 'x', label: 'X', group_id: 'runtime',
        shape: 'nested',
        fields: [{ name: 'a', label: 'A', kind: 'string' }],
      }),
    ).toThrow();
  });
});

describe('ConfigFieldSchema', () => {
  it('accepts a valid datalist_source object', () => {
    const parsed = ConfigFieldSchema.parse({
      name: 'model', label: 'Model', kind: 'string',
      datalist_source: { section: 'providers', field: 'model' },
    });
    expect(parsed.datalist_source).toEqual({ section: 'providers', field: 'model' });
  });

  it('parses kind: multiselect with enum', () => {
    const parsed = ConfigFieldSchema.parse({
      name: 'disabled', label: 'Disabled skills', kind: 'multiselect',
      enum: ['alpha', 'beta'],
    });
    expect(parsed.kind).toBe('multiselect');
  });
});

describe('ProviderModelsResponseSchema', () => {
  it('accepts a valid models list', () => {
    const parsed = ProviderModelsResponseSchema.parse({
      models: ['claude-opus-4-7', 'claude-sonnet-4-6'],
    });
    expect(parsed.models).toEqual(['claude-opus-4-7', 'claude-sonnet-4-6']);
  });

  it('accepts an empty models list', () => {
    const parsed = ProviderModelsResponseSchema.parse({ models: [] });
    expect(parsed.models).toEqual([]);
  });
});

describe('MetaResponseSchema', () => {
  it('parses a full status response', () => {
    const r = MetaResponseSchema.parse({
      version: 'dev',
      uptime_sec: 5,
      storage_driver: 'sqlite',
      instance_root: '/tmp/.hermind',
      current_model: 'anthropic/claude-opus-4-6',
    });
    expect(r.instance_root).toBe('/tmp/.hermind');
  });

  it('rejects missing current_model', () => {
    expect(() => MetaResponseSchema.parse({
      version: 'dev', uptime_sec: 0, storage_driver: 'none', instance_root: '/x',
    })).toThrow();
  });
});

describe('StoredMessageSchema + ConversationHistoryResponseSchema', () => {
  it('parses a history response', () => {
    const r = ConversationHistoryResponseSchema.parse({
      messages: [
        { id: 1, role: 'user', content: '"hi"', timestamp: 1.0 },
        { id: 2, role: 'assistant', content: '"hello"', timestamp: 1.1, finish_reason: 'end_turn' },
      ],
    });
    expect(r.messages).toHaveLength(2);
    expect(r.messages[1]?.finish_reason).toBe('end_turn');
  });

  it('rejects a message missing timestamp', () => {
    expect(() => StoredMessageSchema.parse({
      id: 1, role: 'user', content: 'hi',
    })).toThrow();
  });
});
