import styles from './fields.module.css';
import type { SchemaField } from '../../api/schemas';

export interface FieldProps {
  field: SchemaField;
  value: string;
  onChange: (value: string) => void;
}

export default function TextInput({ field, value, onChange }: FieldProps) {
  return (
    <label className={styles.row}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <input
        type="text"
        className={styles.input}
        value={value}
        placeholder={field.default !== undefined ? String(field.default) : undefined}
        onChange={e => onChange(e.currentTarget.value)}
      />
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
