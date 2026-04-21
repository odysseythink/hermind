import { describe, it, expect } from 'vitest';
import schema from './__fixtures__/config-schema.json';
import enDesc from '../locales/en/descriptors.json';
import zhDesc from '../locales/zh-CN/descriptors.json';

type FlatTree = Record<string, string>;

type SchemaField = {
  name: string;
  kind: string;
  help?: string;
  enum?: string[];
};

type SchemaSection = {
  key: string;
  fields?: SchemaField[];
  group_id?: string;
};

const SECTIONS = schema as unknown as SchemaSection[];

describe.each([
  ['en', enDesc as unknown as FlatTree],
  ['zh-CN', zhDesc as unknown as FlatTree],
])('descriptor translations — %s', (locale, tree) => {
  it('every section has label + summary', () => {
    const missing: string[] = [];
    for (const section of SECTIONS) {
      if (!tree[`${section.key}.label`]) missing.push(`${section.key}.label`);
      if (!tree[`${section.key}.summary`]) missing.push(`${section.key}.summary`);
    }
    expect(missing, `${locale} missing: ${missing.join(', ')}`).toEqual([]);
  });

  it('every field has label (+help when source had it)', () => {
    const missing: string[] = [];
    for (const section of SECTIONS) {
      for (const field of section.fields ?? []) {
        const labelKey = `${section.key}.fields.${field.name}.label`;
        if (!tree[labelKey]) missing.push(labelKey);
        if (field.help) {
          const helpKey = `${section.key}.fields.${field.name}.help`;
          if (!tree[helpKey]) missing.push(helpKey);
        }
      }
    }
    expect(missing, `${locale} missing: ${missing.join(', ')}`).toEqual([]);
  });

  it('every enum field has enum value translations (except empty-string)', () => {
    const missing: string[] = [];
    for (const section of SECTIONS) {
      for (const field of section.fields ?? []) {
        if (field.kind !== 'enum') continue;
        for (const val of field.enum ?? []) {
          if (val === '') continue;
          const key = `${section.key}.fields.${field.name}.enum.${val}`;
          if (!tree[key]) missing.push(key);
        }
      }
    }
    expect(missing, `${locale} missing: ${missing.join(', ')}`).toEqual([]);
  });

  it('every group_id referenced by sections has a groups.<id> entry', () => {
    const ids = new Set(SECTIONS.map((s) => s.group_id).filter(Boolean) as string[]);
    const missing: string[] = [];
    for (const id of ids) {
      if (!tree[`groups.${id}`]) missing.push(`groups.${id}`);
    }
    expect(missing, `${locale} missing: ${missing.join(', ')}`).toEqual([]);
  });
});
