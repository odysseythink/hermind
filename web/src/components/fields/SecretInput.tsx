import { useState } from 'react';
import styles from './SecretInput.module.css';
import type { ConfigField } from '../../api/schemas';
import { useTranslation } from 'react-i18next';

export interface SecretInputProps {
  field: ConfigField;
  value: string;
  // Retained for call-site compatibility. No longer used — reveal only
  // toggles local visibility; there is no server-side reveal endpoint.
  instanceKey?: string;
  dirty?: boolean;
  disableReveal?: boolean;
  onChange: (value: string) => void;
}

export default function SecretInput({
  field,
  value,
  disableReveal,
  onChange,
}: SecretInputProps) {
  const { t } = useTranslation('ui');
  const [revealed, setRevealed] = useState(false);

  return (
    <label className={styles.wrap}>
      <span className={styles.label}>
        {field.label}
        {field.required && <span className={styles.required}>*</span>}
      </span>
      <span className={styles.inputRow}>
        <input
          type={revealed ? 'text' : 'password'}
          className={styles.input}
          value={value}
          placeholder="•••"
          onChange={(e) => {
            setRevealed(false);
            onChange(e.currentTarget.value);
          }}
        />
        <button
          type="button"
          className={styles.revealBtn}
          onClick={() => setRevealed((r) => !r)}
          disabled={Boolean(disableReveal)}
        >
          {revealed ? t('field.hide') : t('field.show')}
        </button>
      </span>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}
