import styles from './fields.module.css';
import type { FieldProps } from './TextInput';

export default function FloatInput({ field, value, onChange }: FieldProps) {
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <input
        type="number"
        step="any"
        className={`${styles.input} ${styles.number}`}
        value={value}
        placeholder={field.default !== undefined ? String(field.default) : undefined}
        onChange={e => onChange(e.currentTarget.value)}
      />
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
