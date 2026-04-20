import styles from './ConfigSection.module.css';
import type { ConfigField, ConfigSection as ConfigSectionT, SchemaField } from '../api/schemas';
import TextInput from './fields/TextInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';
import FloatInput from './fields/FloatInput';

export interface ConfigSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onFieldChange: (name: string, value: unknown) => void;
}

export default function ConfigSection({
  section,
  value,
  originalValue,
  onFieldChange,
}: ConfigSectionProps) {
  return (
    <section className={styles.section} aria-label={section.label}>
      <h2 className={styles.title}>{section.label}</h2>
      {section.summary && <p className={styles.summary}>{section.summary}</p>}
      {section.fields.map(f => {
        if (!isVisible(f, value)) return null;
        const current = asString(value[f.name]);
        const original = asString(originalValue[f.name]);
        const schemaField = f as SchemaField;
        const onChange = (v: string) => onFieldChange(f.name, v);
        switch (f.kind) {
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
          default:
            return <TextInput key={f.name} field={schemaField} value={current} onChange={onChange} />;
        }
      })}
    </section>
  );
}

function isVisible(f: ConfigField, value: Record<string, unknown>): boolean {
  if (!f.visible_when) return true;
  return value[f.visible_when.field] === f.visible_when.equals;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  return String(v);
}
