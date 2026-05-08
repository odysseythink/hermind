import { useState } from 'react';
import type React from 'react';
import styles from './NewMcpServerDialog.module.css';
import { useTranslation } from 'react-i18next';

export interface NewMcpServerDialogProps {
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewMcpServerDialog({
  existingKeys,
  onCancel,
  onCreate,
}: NewMcpServerDialogProps) {
  const { t } = useTranslation('ui');
  const [key, setKey] = useState('');
  const [keyError, setKeyError] = useState<string | null>(null);

  const trimmed = key.trim();
  const duplicate = trimmed !== '' && existingKeys.has(trimmed);
  const formatInvalid = trimmed !== '' && !KEY_REGEX.test(trimmed);
  const disabled = trimmed === '' || duplicate || formatInvalid;

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (disabled) return;
    if (!trimmed) {
      setKeyError(t('error.keyRequired'));
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError(t('error.keyFormat'));
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(t('error.mcpKeyDuplicate', { key: trimmed }));
      return;
    }
    onCreate(trimmed);
  }

  return (
    <div className={styles.overlay} role="dialog" aria-labelledby="newMcpTitle" aria-modal="true">
      <div className={styles.panel}>
        <h2 id="newMcpTitle">{t('dialog.newMcp.title')}</h2>
        <form onSubmit={onSubmit}>
          <label className={styles.row}>
            {t('dialog.newMcp.name')}
            <input
              type="text"
              value={key}
              onChange={e => { setKey(e.currentTarget.value); setKeyError(null); }}
              placeholder="e.g. filesystem"
              autoFocus
            />
          </label>
          {duplicate && (
            <p className={styles.err}>{t('error.mcpKeyDuplicate', { key: trimmed })}</p>
          )}
          {formatInvalid && !duplicate && (
            <p className={styles.err}>{t('error.keyFormat')}</p>
          )}
          {keyError && !duplicate && !formatInvalid && (
            <p className={styles.err}>{keyError}</p>
          )}
          <div className={styles.actions}>
            <button type="button" onClick={onCancel}>{t('action.cancel')}</button>
            <button type="submit" disabled={disabled}>{t('action.create')}</button>
          </div>
        </form>
      </div>
    </div>
  );
}
