import styles from './TopBar.module.css';
import type { Status } from '../../state';

export interface TopBarProps {
  dirtyCount: number;
  status: Status;
  onSave: () => void;
}

export default function TopBar({ dirtyCount, status, onSave }: TopBarProps) {
  const busy = status === 'saving' || status === 'applying';
  const dotClass = busy
    ? styles.dotBusy
    : dirtyCount > 0
      ? styles.dotDirty
      : styles.dotIdle;
  const statusMsg = busy
    ? status === 'saving'
      ? 'Saving…'
      : 'Applying…'
    : dirtyCount > 0
      ? `${dirtyCount} unsaved change${dirtyCount === 1 ? '' : 's'}`
      : 'All saved';
  const saveLabel = dirtyCount > 0 ? `Save · ${dirtyCount} changes` : 'Save';
  return (
    <header className={styles.topbar}>
      <div className={styles.brand}>
        <span className={styles.logo}>⬡</span>
        <span>hermind</span>
      </div>
      <span className={styles.spacer} />
      <span className={styles.status}>
        <span className={`${styles.dot} ${dotClass}`} />
        {statusMsg}
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
