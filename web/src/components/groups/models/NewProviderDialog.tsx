import { useEffect, useRef, useState } from 'react';
import type React from 'react';
import styles from './NewProviderDialog.module.css';
import { useTranslation } from 'react-i18next';

export interface NewProviderDialogProps {
  providerTypes: readonly string[];
  existingKeys: Set<string>;
  onCancel: () => void;
  onCreate: (key: string, providerType: string) => void;
}

const KEY_REGEX = /^[a-z][a-z0-9_]*$/;

export default function NewProviderDialog({
  providerTypes,
  existingKeys,
  onCancel,
  onCreate,
}: NewProviderDialogProps) {
  const { t } = useTranslation('ui');
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [key, setKey] = useState('');
  const [providerType, setProviderType] = useState(providerTypes[0] ?? '');
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    const d = dialogRef.current;
    if (!d) return;
    if (typeof d.showModal === 'function') {
      try { d.showModal(); } catch { d.setAttribute('open', ''); }
    } else {
      d.setAttribute('open', '');
    }
  }, []);

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = key.trim();
    if (!trimmed) {
      setKeyError(t('error.keyRequired'));
      return;
    }
    if (!KEY_REGEX.test(trimmed)) {
      setKeyError(t('error.keyFormat'));
      return;
    }
    if (existingKeys.has(trimmed)) {
      setKeyError(t('error.keyDuplicate', { key: trimmed }));
      return;
    }
    if (!providerType) {
      setKeyError(t('error.providerTypeRequired'));
      return;
    }
    onCreate(trimmed, providerType);
  }

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      onCancel={e => { e.preventDefault(); onCancel(); }}
      onClose={() => onCancel()}
    >
      <form onSubmit={onSubmit}>
        <header className={styles.header}>
          <h2 className={styles.title}>{t('dialog.newProvider.title')}</h2>
          <span className={styles.spacer} />
          <button
            type="button"
            className={styles.close}
            onClick={onCancel}
            aria-label={t('action.close')}
          >✕</button>
        </header>
        <div className={styles.body}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-provider-type">
              {t('dialog.newProvider.type')}
            </label>
            <select
              id="new-provider-type"
              className={styles.select}
              value={providerType}
              onChange={e => setProviderType(e.currentTarget.value)}
            >
              {providerTypes.map(tp => (
                <option key={tp} value={tp}>{tp}</option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="new-provider-key">
              {t('dialog.newInstance.key')}
            </label>
            <input
              id="new-provider-key"
              className={styles.input}
              value={key}
              placeholder="anthropic_main"
              autoFocus
              onChange={e => {
                setKey(e.currentTarget.value);
                setKeyError(null);
              }}
            />
            <span className={styles.hint}>{t('dialog.newProvider.hint')}</span>
            {keyError && <span className={styles.error}>{keyError}</span>}
          </div>
        </div>
        <footer className={styles.footer}>
          <button
            type="button"
            className={`${styles.btn} ${styles.secondary}`}
            onClick={onCancel}
          >{t('action.cancel')}</button>
          <span className={styles.footerSpacer} />
          <button
            type="submit"
            className={`${styles.btn} ${styles.primary}`}
          >{t('action.create')}</button>
        </footer>
      </form>
    </dialog>
  );
}
