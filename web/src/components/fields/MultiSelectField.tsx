import styles from './fields.module.css';
import type { ConfigField } from '../../api/schemas';

export interface MultiSelectFieldProps {
  field: ConfigField;
  value: string[];
  onChange: (next: string[]) => void;
}

export default function MultiSelectField({
  field,
  value,
  onChange,
}: MultiSelectFieldProps) {
  const choices = field.enum ?? [];
  const checked = new Set(value);

  if (choices.length === 0) {
    return (
      <div className={styles.row}>
        <span className={styles.label}>{field.label}</span>
        <span className={styles.help}>
          No skills installed. {field.help}
        </span>
      </div>
    );
  }

  const toggle = (name: string) => {
    const next = new Set(checked);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    onChange(Array.from(next).sort());
  };

  return (
    <fieldset className={styles.row} style={{ border: 'none', padding: 0, margin: 0 }}>
      <legend className={styles.label}>{field.label}</legend>
      <div>
        {choices.map(name => (
          <label key={name} style={{ display: 'flex', gap: '0.5rem' }}>
            <input
              type="checkbox"
              checked={checked.has(name)}
              onChange={() => toggle(name)}
            />
            <span>{name}</span>
          </label>
        ))}
      </div>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </fieldset>
  );
}
