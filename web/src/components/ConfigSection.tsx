import styles from './ConfigSection.module.css';
import type { ConfigField, ConfigSection as ConfigSectionT, SchemaField } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';
import FloatInput from './fields/FloatInput';
import MultiSelectField from './fields/MultiSelectField';
import { getPath } from '../util/path';

export interface ConfigSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onFieldChange: (name: string, value: unknown) => void;
  /** Full config snapshot used to resolve cross-section datalist_source
   *  hints. Optional — fields without datalist_source never need it. */
  config?: Record<string, unknown>;
}

function collectDatalistValues(
  config: Record<string, unknown> | undefined,
  sectionKey: string,
  fieldName: string,
): string[] {
  if (!config) return [];
  const blob = config[sectionKey];
  const out = new Set<string>();
  if (Array.isArray(blob)) {
    for (const el of blob) {
      if (el && typeof el === 'object') {
        const v = (el as Record<string, unknown>)[fieldName];
        if (typeof v === 'string' && v !== '') out.add(v);
      }
    }
  } else if (blob && typeof blob === 'object') {
    for (const el of Object.values(blob as Record<string, unknown>)) {
      if (el && typeof el === 'object') {
        const v = (el as Record<string, unknown>)[fieldName];
        if (typeof v === 'string' && v !== '') out.add(v);
      }
    }
  }
  return Array.from(out).sort();
}

export default function ConfigSection({
  section,
  value,
  originalValue,
  onFieldChange,
  config,
}: ConfigSectionProps) {
  return (
    <section className={styles.section} aria-label={section.label}>
      <h2 className={styles.title}>{section.label}</h2>
      {section.summary && <p className={styles.summary}>{section.summary}</p>}
      {section.fields.map(f => {
        if (!isVisible(f, value)) return null;
        const current = asString(getPath(value, f.name));
        const original = asString(getPath(originalValue, f.name));
        const schemaField = f as SchemaField;
        const onChange = (v: string) => onFieldChange(f.name, v);
        switch (f.kind) {
          case 'multiselect': {
            const raw = getPath(value, f.name);
            const arr = Array.isArray(raw)
              ? (raw as unknown[]).filter((x): x is string => typeof x === 'string')
              : [];
            return (
              <MultiSelectField
                key={f.name}
                field={f}
                value={arr}
                onChange={(next: string[]) => onFieldChange(f.name, next)}
              />
            );
          }
          case 'int':
            return <NumberInput key={f.name} field={schemaField} value={current} onChange={onChange} />;
          case 'float':
            return <FloatInput key={f.name} field={schemaField} value={current} onChange={onChange} />;
          case 'bool':
            return <BoolToggle key={f.name} field={schemaField} value={current} onChange={onChange} />;
          case 'enum':
            return <EnumSelect key={f.name} field={schemaField} value={current} onChange={onChange} />;
          case 'secret':
            return (
              <SecretInput
                key={f.name}
                field={schemaField}
                value={current}
                instanceKey=""
                dirty={current !== original}
                disableReveal
                onChange={onChange}
              />
            );
          case 'string':
          default: {
            const suggestions = f.datalist_source
              ? collectDatalistValues(config, f.datalist_source.section, f.datalist_source.field)
              : undefined;
            return (
              <TextInput
                key={f.name}
                field={schemaField}
                value={current}
                onChange={onChange}
                datalist={suggestions}
              />
            );
          }
        }
      })}
    </section>
  );
}

function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  // Values arrive as real types on boot (bool true, number 42) but edited
  // values pass through string-coerced field onChange handlers. Coerce both
  // sides to string so predicates keep matching either way. getPath handles
  // both flat and dotted discriminator names (e.g. "provider" or "foo.bar").
  return String(getPath(value, f.visible_when.field)) === String(f.visible_when.equals);
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  return String(v);
}
