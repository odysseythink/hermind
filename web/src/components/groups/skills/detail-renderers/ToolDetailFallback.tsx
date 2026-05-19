import { useMemo } from 'react';
import pageStyles from '../SkillToolsConfigPage.module.css';
import styles from './ToolDetailFallback.module.css';
import Switch from '../../../fields/Switch';
import type { ConfigField, ConfigPredicate } from '../../../api/schemas';
import type { ToolDetailProps } from './types';

function asBool(v: unknown): boolean {
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v === 'true';
  return false;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

function parseIntField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

function parseFloatField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

function getToolSettingValue(
  toolName: string,
  fieldName: string,
  config?: Record<string, unknown>,
): unknown {
  const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
    | Record<string, Record<string, unknown>>
    | undefined);
  return settings?.[toolName]?.[fieldName];
}

function setToolSettingValue(
  toolName: string,
  fieldName: string,
  value: unknown,
  config: Record<string, unknown> | undefined,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  if (!onSectionField) return;
  const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
  const settings = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
  const nextToolSettings = { ...(settings[toolName] ?? {}), [fieldName]: value };
  const nextSettings = { ...settings, [toolName]: nextToolSettings };
  onSectionField('tools', 'settings', nextSettings);
}

function evaluatePredicate(
  predicate: ConfigPredicate | undefined,
  values: Record<string, unknown>,
): boolean {
  if (!predicate) return true;
  const val = values[predicate.field];
  if (predicate.equals !== undefined) {
    return val === predicate.equals;
  }
  if (predicate.in !== undefined) {
    return predicate.in.includes(val);
  }
  return true;
}

function groupBy<T>(arr: T[], keyFn: (item: T) => string): Record<string, T[]> {
  const groups: Record<string, T[]> = {};
  for (const item of arr) {
    const key = keyFn(item);
    if (!groups[key]) groups[key] = [];
    groups[key].push(item);
  }
  return groups;
}

export default function ToolDetailFallback({
  name,
  description,
  toolset,
  enabled,
  settings_schema,
  onToggle,
  config,
  onSectionField,
}: ToolDetailProps) {
  const values = useMemo(() => {
    const v: Record<string, unknown> = {};
    if (settings_schema) {
      for (const field of settings_schema) {
        v[field.name] = getToolSettingValue(name, field.name, config);
      }
    }
    return v;
  }, [name, settings_schema, config]);

  const groups = useMemo(() => {
    if (!settings_schema) return {};
    return groupBy(settings_schema, (f) => f.group || 'General');
  }, [settings_schema]);

  const groupOrder = useMemo(() => {
    const keys = Object.keys(groups);
    const generalIdx = keys.indexOf('General');
    if (generalIdx > 0) {
      keys.splice(generalIdx, 1);
      keys.unshift('General');
    }
    return keys;
  }, [groups]);

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          {name}
          {toolset && (
            <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>
      {description && <div className={pageStyles.detailDesc}>{description}</div>}
      <div className={pageStyles.configSection}>
        {settings_schema && settings_schema.length > 0 ? (
          groupOrder.map((groupName) => {
            const fields = groups[groupName].filter((f) => evaluatePredicate(f.visible_when, values));
            if (fields.length === 0) return null;
            return (
              <div key={groupName}>
                {groupName !== 'General' && <h3 className={styles.groupTitle}>{groupName}</h3>}
                {fields.map((field) => (
                  <SchemaFieldRow
                    key={field.name}
                    field={field}
                    toolName={name}
                    value={values[field.name]}
                    config={config}
                    onSectionField={onSectionField}
                  />
                ))}
              </div>
            );
          })
        ) : (
          <p className={styles.noSettings}>此工具暂无配置项。</p>
        )}
      </div>
    </div>
  );
}

function SchemaFieldRow({
  field,
  toolName,
  value,
  config,
  onSectionField,
}: {
  field: ConfigField;
  toolName: string;
  value: unknown;
  config: Record<string, unknown> | undefined;
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void;
}) {
  const label = field.label || field.name;
  const help = field.help || '';

  const handleChange = (next: unknown) => {
    setToolSettingValue(toolName, field.name, next, config, onSectionField);
  };

  let input: React.ReactNode;

  if (field.kind === 'bool') {
    input = <Switch checked={asBool(value)} onChange={handleChange} ariaLabel={label} />;
  } else if (field.kind === 'int') {
    input = (
      <input
        type="number"
        className={styles.numberInput}
        value={asString(value)}
        onChange={(e) => handleChange(parseIntField(e.currentTarget.value))}
        aria-label={label}
      />
    );
  } else if (field.kind === 'float') {
    input = (
      <input
        type="number"
        step="any"
        className={styles.numberInput}
        value={asString(value)}
        onChange={(e) => handleChange(parseFloatField(e.currentTarget.value))}
        aria-label={label}
      />
    );
  } else if (field.kind === 'secret') {
    input = (
      <input
        type="password"
        className={styles.input}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  } else if (field.kind === 'enum') {
    const choices = field.enum ?? [];
    input = (
      <select
        className={styles.select}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        aria-label={label}
      >
        {!field.required && <option value="">—</option>}
        {choices.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
    );
  } else if (field.kind === 'text') {
    input = (
      <textarea
        className={styles.textarea}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  } else if (field.kind === 'multiselect') {
    const current = Array.isArray(value) ? value : [];
    const choices = field.enum ?? [];
    input = (
      <div className={styles.multiSelect}>
        {choices.map((c) => (
          <label key={c} className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={current.includes(c)}
              onChange={(e) => {
                const next = e.currentTarget.checked
                  ? [...current, c]
                  : current.filter((x: string) => x !== c);
                handleChange(next);
              }}
            />
            <span>{c}</span>
          </label>
        ))}
      </div>
    );
  } else {
    input = (
      <input
        type="text"
        className={styles.input}
        value={asString(value)}
        onChange={(e) => handleChange(e.currentTarget.value)}
        placeholder={field.default as string}
        aria-label={label}
      />
    );
  }

  return (
    <div className={styles.configRow}>
      <div>
        <div className={styles.label}>{label}</div>
        {help && <div className={styles.help}>{help}</div>}
      </div>
      {input}
    </div>
  );
}
