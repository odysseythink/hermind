import styles from './TopBar.module.css';

export interface TopBarProps {
  dirtyCount: number;
  status: 'booting' | 'ready' | 'saving' | 'applying' | 'error';
}

export default function TopBar({ dirtyCount, status }: TopBarProps) {
  const dotClass =
    status === 'saving' || status === 'applying'
      ? styles.dotBusy
      : dirtyCount > 0
        ? styles.dotDirty
        : styles.dotIdle;
  const msg =
    status === 'saving'
      ? 'Saving…'
      : status === 'applying'
        ? 'Applying…'
        : dirtyCount > 0
          ? `${dirtyCount} unsaved change${dirtyCount === 1 ? '' : 's'}`
          : 'All saved';
  return (
    <header className={styles.topbar}>
      <div className={styles.brand}>
        <span className={styles.logo}>⬡</span>
        <span className={styles.title}>hermind</span>
      </div>
      <span className={styles.spacer} />
      <span className={styles.status}>
        <span className={dotClass} />
        {msg}
      </span>
    </header>
  );
}
