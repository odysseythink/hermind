import styles from './ModelsSidebar.module.css';

export interface ModelsSidebarProps {
  instances: Array<{ key: string; type: string }>;
  activeSubKey: string | null;
  dirtyKeys: Set<string>;
  onSelectScalar: (key: string) => void;
  onSelectInstance: (key: string) => void;
  onNewProvider: () => void;
}

export default function ModelsSidebar({
  instances,
  activeSubKey,
  dirtyKeys,
  onSelectScalar,
  onSelectInstance,
  onNewProvider,
}: ModelsSidebarProps) {
  return (
    <div className={styles.sidebar}>
      <button
        type="button"
        className={`${styles.scalarRow} ${activeSubKey === 'model' ? styles.active : ''}`}
        onClick={() => onSelectScalar('model')}
      >
        Default model
      </button>
      <div className={styles.groupHeader}>Providers</div>
      {instances.length === 0 && (
        <div className={styles.empty}>No providers configured.</div>
      )}
      {instances.map(inst => (
        <button
          key={inst.key}
          type="button"
          className={`${styles.item} ${inst.key === activeSubKey ? styles.active : ''}`}
          onClick={() => onSelectInstance(inst.key)}
        >
          <span className={styles.itemRow}>
            <span className={styles.itemKey}>{inst.key}</span>
            {dirtyKeys.has(inst.key) && (
              <span className={styles.dirtyDot} title="Unsaved changes" />
            )}
          </span>
          <span className={styles.itemType}>{inst.type}</span>
        </button>
      ))}
      <button type="button" className={styles.newBtn} onClick={onNewProvider}>
        + New provider
      </button>
    </div>
  );
}
