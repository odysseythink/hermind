import { useState } from 'react';
import type React from 'react';
import styles from './NewPlatformDialog.module.css';
import { useTranslation } from 'react-i18next';

export interface NewPlatformDialogProps {
  existingKeys: Set<string>;
  platformTypes: string[];
  onCancel: () => void;
  onCreate: (key: string, type: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewPlatformDialog({
  existingKeys,
  platformTypes,
  onCancel,
  onCreate,
}: NewPlatformDialogProps) {
  const { t } = useTranslation('ui');
  const [key, setKey] = useState('');
  const [type, setType] = useState(platformTypes[0] || '');
  const [keyError, setKeyError] = useState<string | null>(null);

  const trimmedKey = key.trim();
  const duplicate = trimmedKey !== '' && existingKeys.has(trimmedKey);
  const formatInvalid = trimmedKey !== '' && !KEY_REGEX.test(trimmedKey);
  const disabled = trimmedKey === '' || type === '' || duplicate || formatInvalid;

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (disabled) return;
    if (!trimmedKey) {
      setKeyError(t('error.keyRequired'));
      return;
    }
    if (!KEY_REGEX.test(trimmedKey)) {
      setKeyError(t('error.keyFormat'));
      return;
    }
    if (existingKeys.has(trimmedKey)) {
      setKeyError(t('error.platformKeyDuplicate', { key: trimmedKey }));
      return;
    }
    onCreate(trimmedKey, type);
  }

  return (
    <div className={styles.overlay} role="dialog" aria-labelledby="newPlatformTitle" aria-modal="true">
      <div className={styles.panel}>
        <h2 id="newPlatformTitle">{t('dialog.newPlatform.title')}</h2>
        <form onSubmit={onSubmit}>
          <label className={styles.row}>
            {t('dialog.newPlatform.type')}
            <select value={type} onChange={e => setType(e.currentTarget.value)}>
              <option value="">{t('dialog.newPlatform.selectType')}</option>
              {platformTypes.map(pt => (
                <option key={pt} value={pt}>
                  {pt}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.row}>
            {t('dialog.newPlatform.key')}
            <input
              type="text"
              value={key}
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
              placeholder={t('dialog.newPlatform.keyPlaceholder')}
              autoFocus
            />
          </label>
          {duplicate && (
            <p className={styles.err}>{t('error.platformKeyDuplicate', { key: trimmedKey })}</p>
          )}
          {formatInvalid && !duplicate && (
            <p className={styles.err}>{t('error.keyFormat')}</p>
          )}
          {keyError && !duplicate && !formatInvalid && (
            <p className={styles.err}>{keyError}</p>
          )}
          <div className={styles.actions}>
            <button type="button" onClick={onCancel}>
              {t('action.cancel')}
            </button>
            <button type="submit" disabled={disabled}>
              {t('action.create')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
