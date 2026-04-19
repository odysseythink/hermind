import styles from './Footer.module.css';
import type { Flash } from '../state';

export interface FooterProps {
  dirtyCount: number;
  flash: Flash | null;
  busy: boolean;
  onSave: () => void;
  onSaveAndApply: () => void;
}

export default function Footer({
  dirtyCount,
  flash,
  busy,
  onSave,
  onSaveAndApply,
}: FooterProps) {
  const flashClass =
    flash?.kind === 'err' ? styles.flashErr : flash?.kind === 'ok' ? styles.flashOk : '';
  const label = flash?.msg ?? (dirtyCount > 0 ? `${dirtyCount} unsaved` : '');
  return (
    <footer className={styles.footer}>
      <span className={`${styles.status} ${flashClass}`}>{label}</span>
      <span className={styles.spacer} />
      <button
        type="button"
        className={`${styles.btn} ${styles.secondary}`}
        onClick={onSave}
        disabled={busy || dirtyCount === 0}
      >
        Save
      </button>
      <button
        type="button"
        className={`${styles.btn} ${styles.primary}`}
        onClick={onSaveAndApply}
        disabled={busy || dirtyCount === 0}
      >
        Save &amp; Apply
      </button>
    </footer>
  );
}
