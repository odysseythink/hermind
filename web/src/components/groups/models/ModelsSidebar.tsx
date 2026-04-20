import styles from './ModelsSidebar.module.css';

export interface ModelsSidebarProps {
  instances: Array<{ key: string; type: string }>;
  activeSubKey: string | null;
  dirtyKeys: Set<string>;
  onSelectScalar: (key: string) => void;
  onSelectInstance: (key: string) => void;
  onNewProvider: () => void;
  // Stage 4c additions
  fallbackProviders: Array<{ provider: string }>;
  dirtyFallbackIndices: Set<number>;
  activeFallbackIndex: number | null;
  onSelectFallback: (index: number) => void;
  onAddFallback: () => void;
  onMoveFallback: (index: number, direction: 'up' | 'down') => void;
}

export default function ModelsSidebar({
  instances,
  activeSubKey,
  dirtyKeys,
  onSelectScalar,
  onSelectInstance,
  onNewProvider,
  fallbackProviders,
  dirtyFallbackIndices,
  activeFallbackIndex,
  onSelectFallback,
  onAddFallback,
  onMoveFallback,
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
      <div className={styles.groupHeader}>Fallback Providers</div>
      {fallbackProviders.length === 0 && (
        <div className={styles.empty}>No fallback providers configured.</div>
      )}
      {fallbackProviders.map((fb, i) => {
        const active = i === activeFallbackIndex;
        const atTop = i === 0;
        const atBottom = i === fallbackProviders.length - 1;
        return (
          <div
            key={i}
            className={`${styles.fallbackRow} ${active ? styles.active : ''}`}
          >
            <button
              type="button"
              className={styles.fallbackBody}
              onClick={() => onSelectFallback(i)}
            >
              <span className={styles.fallbackRowInner}>
                <span className={styles.posBadge}>#{i + 1}</span>
                <span className={styles.fallbackType}>{fb.provider}</span>
                {dirtyFallbackIndices.has(i) && (
                  <span className={styles.dirtyDot} title="Unsaved changes" />
                )}
              </span>
            </button>
            <div className={styles.fallbackMoveBtns}>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move up"
                disabled={atTop}
                onClick={() => onMoveFallback(i, 'up')}
              >
                ↑
              </button>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move down"
                disabled={atBottom}
                onClick={() => onMoveFallback(i, 'down')}
              >
                ↓
              </button>
            </div>
          </div>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddFallback}>
        + Add fallback
      </button>
    </div>
  );
}
