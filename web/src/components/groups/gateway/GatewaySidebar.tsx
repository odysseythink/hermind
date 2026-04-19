import styles from './GatewaySidebar.module.css';
import type { SchemaDescriptor } from '../../../api/schemas';

export interface GatewaySidebarProps {
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  dirtyKeys: Set<string>;
  onSelect: (key: string) => void;
  onNewInstance: () => void;
}

export default function GatewaySidebar({
  instances,
  selectedKey,
  descriptors,
  dirtyKeys,
  onSelect,
  onNewInstance,
}: GatewaySidebarProps) {
  const displayNames = new Map(descriptors.map(d => [d.type, d.display_name]));
  return (
    <div className={styles.sidebar}>
      {instances.length === 0 && (
        <div className={styles.empty}>No instances configured.</div>
      )}
      {instances.map(inst => (
        <button
          key={inst.key}
          type="button"
          className={`${styles.item} ${inst.key === selectedKey ? styles.active : ''} ${!inst.enabled ? styles.dimmed : ''}`}
          onClick={() => onSelect(inst.key)}
        >
          <span className={styles.itemRow}>
            <span className={styles.itemKey}>{inst.key}</span>
            {dirtyKeys.has(inst.key) && (
              <span className={styles.dirtyDot} title="Unsaved changes" />
            )}
          </span>
          <span className={styles.itemType}>
            {displayNames.get(inst.type) ?? inst.type}
            {!inst.enabled && <span className={styles.offBadge}>off</span>}
          </span>
        </button>
      ))}
      <button type="button" className={styles.newBtn} onClick={onNewInstance}>
        + New instance
      </button>
    </div>
  );
}
