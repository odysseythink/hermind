import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SessionSummary, SessionPatch } from '../../api/schemas';
import styles from './SessionSettingsDrawer.module.css';

type Props = {
  open: boolean;
  session: SessionSummary;
  modelOptions: string[];
  onClose: () => void;
  onSave: (patch: SessionPatch) => Promise<void>;
};

export default function SessionSettingsDrawer({
  open, session, modelOptions, onClose, onSave,
}: Props) {
  const { t } = useTranslation('ui');
  const [draftModel, setDraftModel] = useState(session.model ?? '');
  const [draftPrompt, setDraftPrompt] = useState(session.system_prompt ?? '');
  const [savedModel, setSavedModel] = useState(session.model ?? '');
  const [savedPrompt, setSavedPrompt] = useState(session.system_prompt ?? '');
  const [saving, setSaving] = useState(false);
  const textAreaRef = useRef<HTMLTextAreaElement>(null);

  // Reset draft + saved baselines when drawer opens or session id changes.
  useEffect(() => {
    if (open) {
      setDraftModel(session.model ?? '');
      setDraftPrompt(session.system_prompt ?? '');
      setSavedModel(session.model ?? '');
      setSavedPrompt(session.system_prompt ?? '');
      // Focus the prompt after mount
      setTimeout(() => textAreaRef.current?.focus(), 0);
    }
  }, [open, session.id]);

  // Track whether the session prop diverged from our baseline (another tab).
  const sessionModel = session.model ?? '';
  const sessionPrompt = session.system_prompt ?? '';
  const draftDirty = draftModel !== savedModel || draftPrompt !== savedPrompt;
  const externalChange =
    (sessionModel !== savedModel || sessionPrompt !== savedPrompt) && draftDirty;

  function handleCancel() {
    setDraftModel(savedModel);
    setDraftPrompt(savedPrompt);
    onClose();
  }

  async function handleSave() {
    if (saving || !draftDirty) return;
    const patch: SessionPatch = {};
    if (draftModel !== savedModel) patch.model = draftModel;
    if (draftPrompt !== savedPrompt) patch.system_prompt = draftPrompt;
    setSaving(true);
    try {
      await onSave(patch);
      setSavedModel(draftModel);
      setSavedPrompt(draftPrompt);
      onClose();
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t('chat.settings.title')}
      className={styles.drawer}
      onKeyDown={(e) => {
        if (e.key === 'Escape') handleCancel();
      }}
      tabIndex={-1}
    >
      <header className={styles.header}>
        <h3 className={styles.title}>{t('chat.settings.title')}</h3>
      </header>

      {externalChange && (
        <div role="status" className={styles.conflict}>
          {t('chat.settings.updatedElsewhere')}
        </div>
      )}

      <label className={styles.field}>
        <span className={styles.label}>{t('chat.settings.model')}</span>
        <select
          className={styles.select}
          value={draftModel}
          onChange={(e) => setDraftModel(e.target.value)}
        >
          {modelOptions.map((m) => (
            <option key={m || '(default)'} value={m}>
              {m || t('chat.settings.defaultModel')}
            </option>
          ))}
        </select>
      </label>

      <label className={styles.field}>
        <span className={styles.label}>{t('chat.settings.systemPrompt')}</span>
        <textarea
          ref={textAreaRef}
          className={styles.textarea}
          value={draftPrompt}
          onChange={(e) => setDraftPrompt(e.target.value)}
          rows={12}
        />
      </label>

      <footer className={styles.actions}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleCancel}
          disabled={saving}
        >
          {t('chat.settings.cancel')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.primary}`}
          onClick={handleSave}
          disabled={saving || !draftDirty}
        >
          {t('chat.settings.save')}
        </button>
      </footer>
    </div>
  );
}
