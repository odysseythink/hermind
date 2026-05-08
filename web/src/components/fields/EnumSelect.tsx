import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function EnumSelect({ field, value, onChange }: FieldProps) {
  const choices = field.enum ?? [];
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <select
        className={styles.select}
        value={value}
        onChange={e => onChange(e.currentTarget.value)}
      >
        {!field.required && <option value="">—</option>}
        {choices.map(c => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
