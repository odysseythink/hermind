import { useState } from 'react';
import styles from './SecretInput.module.css';
import type { SchemaField } from '../../api/schemas';
import { RevealResponseSchema } from '../../api/schemas';
import { apiFetch, ApiError } from '../../api/client';
import { useTranslation } from 'react-i18next';

export interface SecretInputProps {
  field: SchemaField;
  value: string;
  instanceKey: string;
  dirty: boolean;
  disableReveal?: boolean;
  onChange: (value: string) => void;
}

export default function SecretInput({
  field,
  value,
  instanceKey,
  dirty,
  disableReveal,
  onChange,
}: SecretInputProps) {
  const { t } = useTranslation('ui');
  const [revealed, setRevealed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onToggle() {
    if (revealed) {
      setRevealed(false);
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const res = await apiFetch(
        `/api/platforms/${encodeURIComponent(instanceKey)}/reveal`,
        {
          method: 'POST',
          body: { field: field.name },
          schema: RevealResponseSchema,
        },
      );
      onChange(res.value);
      setRevealed(true);
    } catch (e) {
      setErr(toMsg(e));
    } finally {
      setBusy(false);
    }
  }

  const showDisabled = busy || dirty || Boolean(disableReveal);
  const showTitle = disableReveal
    ? t('field.revealDisabled')
    : dirty
      ? t('field.saveBeforeReveal')
      : undefined;

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
          onChange={e => {
            setRevealed(false);
            onChange(e.currentTarget.value);
          }}
        />
        <button
          type="button"
          className={styles.revealBtn}
          onClick={onToggle}
          disabled={showDisabled}
          title={showTitle}
        >
          {busy ? '…' : revealed ? t('field.hide') : t('field.show')}
        </button>
      </span>
      {err && <span className={styles.error}>{err}</span>}
      {field.help && !err && <span className={styles.help}>{field.help}</span>}
    </label>
  );
}

function toMsg(e: unknown): string {
  if (e instanceof ApiError) {
    if (typeof e.body === 'object' && e.body !== null && 'error' in e.body) {
      const m = (e.body as { error?: unknown }).error;
      if (typeof m === 'string') return m;
    }
    return `HTTP ${e.status}`;
  }
  return e instanceof Error ? e.message : String(e);
}
