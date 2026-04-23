import styles from './ConfigSection.module.css';
import type { ConfigField, ConfigSection as ConfigSectionT } from '../api/schemas';
import TextInput from './fields/TextInput';
import TextAreaInput from './fields/TextAreaInput';
import NumberInput from './fields/NumberInput';
import BoolToggle from './fields/BoolToggle';
import EnumSelect from './fields/EnumSelect';
import SecretInput from './fields/SecretInput';
import FloatInput from './fields/FloatInput';
import MultiSelectField from './fields/MultiSelectField';
import { getPath } from '../util/path';
import { useDescriptorT } from '../i18n/useDescriptorT';

export interface ConfigSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onFieldChange: (name: string, value: unknown) => void;
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

function localizeField(
  section: ConfigSectionT,
  field: ConfigField,
  dt: ReturnType<typeof useDescriptorT>,
): ConfigField {
  return {
    ...field,
    label: dt.fieldLabel(section.key, field.name, field.label),
    help: field.help ? dt.fieldHelp(section.key, field.name, field.help) : field.help,
  };
}

export default function ConfigSection({
  section,
  value,
  originalValue,
  onFieldChange,
  config,
}: ConfigSectionProps) {
  const dt = useDescriptorT();
  const sectionLabel = dt.sectionLabel(section.key, section.label);
  const sectionSummary = section.summary
    ? dt.sectionSummary(section.key, section.summary)
    : '';
  return (
    <section className={styles.section} aria-label={sectionLabel}>
      <h2 className={styles.title}>{sectionLabel}</h2>
      {sectionSummary && <p className={styles.summary}>{sectionSummary}</p>}
      {section.fields.map(f => {
        if (!isVisible(f, value)) return null;
        const current = asString(getPath(value, f.name));
        const original = asString(getPath(originalValue, f.name));
        const localized = localizeField(section, f, dt);
        const schemaField = localized as ConfigField;
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
                field={localized}
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
          case 'text':
            return (
              <TextAreaInput
                key={f.name}
                value={current}
                onChange={(v) => onFieldChange(f.name, v)}
                placeholder={localized.help ?? ''}
                aria-label={localized.label}
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
  return String(getPath(value, f.visible_when.field)) === String(f.visible_when.equals);
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  return String(v);
}
