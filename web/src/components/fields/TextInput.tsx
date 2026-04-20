import { useId } from 'react';
import styles from './fields.module.css';
import type { SchemaField } from '../../api/schemas';

export interface FieldProps {
  field: SchemaField;
  value: string;
  onChange: (value: string) => void;
  /** Optional autocomplete suggestions. When non-empty, a sibling <datalist>
   *  renders and the input's list= attribute wires up to it. */
  datalist?: readonly string[];
}

export default function TextInput({ field, value, onChange, datalist }: FieldProps) {
  const listId = useId();
  const hasList = Array.isArray(datalist) && datalist.length > 0;
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
        list={hasList ? listId : undefined}
      />
      {hasList && (
        <datalist id={listId}>
          {datalist!.map(v => (
            <option key={v} value={v} />
          ))}
        </datalist>
      )}
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
