import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function BoolToggle({ field, value, onChange }: FieldProps) {
  const checked = value === 'true';
  return (
    <label className={styles.toggleRow}>
      <input
        type="checkbox"
        checked={checked}
        onChange={e => onChange(e.currentTarget.checked ? 'true' : 'false')}
      />
      <span>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
