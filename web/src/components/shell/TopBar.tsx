import styles from './TopBar.module.css';
import type { Status } from '../../state';
import LanguageToggle from './LanguageToggle';
import ThemeToggle from './ThemeToggle';
import { useTranslation } from 'react-i18next';

export type ShellMode = 'chat' | 'settings';

export interface TopBarProps {
  dirtyCount: number;
  status: Status;
  onSave: () => void;
  mode?: ShellMode;
  onModeChange?: (m: ShellMode) => void;
}

export default function TopBar({ dirtyCount, status, onSave, mode = 'settings', onModeChange }: TopBarProps) {
  const { t } = useTranslation('ui');
  const busy = status === 'saving' || status === 'applying';
  const dotClass = busy
    ? styles.dotBusy
    : dirtyCount > 0
      ? styles.dotDirty
      : styles.dotIdle;
  const statusMsg = busy
    ? status === 'saving'
      ? t('action.saving')
      : t('action.applying')
    : dirtyCount > 0
      ? t('status.unsavedChanges', { count: dirtyCount })
      : t('status.allSaved');
  const saveLabel = dirtyCount > 0
    ? t('status.saveWithCount', { count: dirtyCount })
    : t('action.save');
  return (
    <header className={styles.topbar}>
      <div className={styles.brand}>
        <span className={styles.logo}>⬡</span>
        <span>hermind</span>
      </div>
      {onModeChange && (
        <div className={styles.modeToggle} role="group" aria-label={t('mode.switcher')}>
          <button
            type="button"
            aria-pressed={mode === 'chat'}
            onClick={() => onModeChange('chat')}
          >{t('mode.chat')}</button>
          <button
            type="button"
            aria-pressed={mode === 'settings'}
            onClick={() => onModeChange('settings')}
          >{t('mode.settings')}</button>
        </div>
      )}
      <span className={styles.spacer} />
      <span className={styles.status}>
        <span className={`${styles.dot} ${dotClass}`} />
        {statusMsg}
      </span>
      <span className={styles.langSlot}>
        <LanguageToggle />
      </span>
      <span className={styles.themeSlot}>
        <ThemeToggle />
      </span>
      <button
        type="button"
        className={styles.saveBtn}
        onClick={onSave}
        disabled={busy || dirtyCount === 0}
      >
        {saveLabel}
      </button>
    </header>
  );
}
