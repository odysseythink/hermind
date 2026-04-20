import { describe, expect, it } from 'vitest';
import {
  ApplyResultSchema,
  ConfigResponseSchema,
  ConfigSchemaResponseSchema,
  ConfigSectionSchema,
  FieldKindSchema,
  PlatformTestResponseSchema,
  PlatformsSchemaResponseSchema,
  RevealResponseSchema,
  SchemaDescriptorSchema,
  SchemaFieldSchema,
} from './schemas';

describe('FieldKindSchema', () => {
  it('accepts every known kind string', () => {
    for (const k of ['unknown', 'string', 'int', 'bool', 'secret', 'enum']) {
      expect(() => FieldKindSchema.parse(k)).not.toThrow();
    }
  });

  it('rejects unknown values', () => {
    expect(() => FieldKindSchema.parse('float')).toThrow();
  });
});

describe('SchemaFieldSchema', () => {
  it('parses a minimal field', () => {
    const field = SchemaFieldSchema.parse({
      name: 'token',
      label: 'Token',
      kind: 'secret',
    });
    expect(field.required).toBeUndefined();
    expect(field.enum).toBeUndefined();
  });

  it('passes through required + enum', () => {
    const field = SchemaFieldSchema.parse({
      name: 'region',
      label: 'Region',
      kind: 'enum',
      required: true,
      enum: ['us', 'eu'],
    });
    expect(field.required).toBe(true);
    expect(field.enum).toEqual(['us', 'eu']);
  });

  it('rejects a missing kind', () => {
    expect(() =>
      SchemaFieldSchema.parse({ name: 'x', label: 'X' }),
    ).toThrow();
  });
});

describe('SchemaDescriptorSchema', () => {
  it('parses a descriptor with zero fields', () => {
    const d = SchemaDescriptorSchema.parse({
      type: 'echo',
      display_name: 'Echo',
      fields: [],
    });
    expect(d.summary).toBeUndefined();
  });

  it('threads fields through', () => {
    const d = SchemaDescriptorSchema.parse({
      type: 'telegram',
      display_name: 'Telegram Bot',
      fields: [
        { name: 'token', label: 'Token', kind: 'secret', required: true },
      ],
    });
    expect(d.fields).toHaveLength(1);
    expect(d.fields[0]?.kind).toBe('secret');
  });
});

describe('PlatformsSchemaResponseSchema', () => {
  it('parses an empty descriptors list', () => {
    const r = PlatformsSchemaResponseSchema.parse({ descriptors: [] });
    expect(r.descriptors).toEqual([]);
  });
});

describe('ConfigResponseSchema', () => {
  it('accepts an unknown top-level key (catchall unknown)', () => {
    const r = ConfigResponseSchema.parse({
      config: {
        model: 'claude-sonnet-4-5',
        providers: { anthropic: { api_key: 'redacted' } },
        gateway: {
          platforms: {
            tg: { enabled: true, type: 'telegram', options: { token: '' } },
          },
        },
      },
    });
    expect(r.config.gateway?.platforms?.tg?.type).toBe('telegram');
    expect((r.config as Record<string, unknown>).model).toBe('claude-sonnet-4-5');
  });

  it('parses a config with no gateway block', () => {
    const r = ConfigResponseSchema.parse({
      config: { model: 'claude-sonnet-4-5' },
    });
    expect(r.config.gateway).toBeUndefined();
  });
});

describe('ApplyResultSchema', () => {
  it('parses a minimal ok response', () => {
    const r = ApplyResultSchema.parse({ ok: true, took_ms: 42 });
    expect(r.error).toBeUndefined();
  });

  it('parses a failure with per-key errors', () => {
    const r = ApplyResultSchema.parse({
      ok: false,
      took_ms: 300,
      error: 'some failed',
      errors: { tg_main: 'HTTP 401' },
    });
    expect(r.errors?.tg_main).toBe('HTTP 401');
  });

  it('rejects a missing took_ms', () => {
    expect(() => ApplyResultSchema.parse({ ok: true })).toThrow();
  });
});

describe('PlatformTestResponseSchema', () => {
  it('parses ok=true', () => {
    expect(PlatformTestResponseSchema.parse({ ok: true })).toEqual({ ok: true });
  });

  it('parses ok=false with error', () => {
    const r = PlatformTestResponseSchema.parse({ ok: false, error: 'oops' });
    expect(r.error).toBe('oops');
  });
});

describe('RevealResponseSchema', () => {
  it('parses a value', () => {
    expect(RevealResponseSchema.parse({ value: 'abc' })).toEqual({ value: 'abc' });
  });

  it('rejects a missing value', () => {
    expect(() => RevealResponseSchema.parse({})).toThrow();
  });
});

describe('ConfigSchemaResponseSchema', () => {
  it('accepts a storage section with visible_when', () => {
    const good = {
      sections: [
        {
          key: 'storage',
          label: 'Storage',
          summary: 'Where hermind keeps data.',
          group_id: 'runtime',
          fields: [
            { name: 'driver', label: 'Driver', kind: 'enum',
              required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
            { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
              visible_when: { field: 'driver', equals: 'sqlite' } },
            { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
              visible_when: { field: 'driver', equals: 'postgres' } },
          ],
        },
      ],
    };
    expect(() => ConfigSchemaResponseSchema.parse(good)).not.toThrow();
  });

  it('rejects a response missing sections', () => {
    const bad = { whatever: [] };
    expect(() => ConfigSchemaResponseSchema.parse(bad)).toThrow();
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
        key: 'x',
        label: 'X',
        group_id: 'runtime',
        shape: 'nested', // unknown
        fields: [{ name: 'a', label: 'A', kind: 'string' }],
      }),
    ).toThrow();
  });
});
