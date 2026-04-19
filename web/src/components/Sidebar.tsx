import styles from './Sidebar.module.css';
import type { SchemaDescriptor } from '../api/schemas';

export interface SidebarProps {
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  onSelect: (key: string) => void;
  onNewInstance: () => void;
}

export default function Sidebar({
  instances,
  selectedKey,
  descriptors,
  onSelect,
  onNewInstance,
}: SidebarProps) {
  const displayNames = new Map(descriptors.map(d => [d.type, d.display_name]));
  return (
    <aside className={styles.sidebar}>
      <div className={styles.label}>Messaging Platforms</div>
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
          <span className={styles.itemKey}>{inst.key}</span>
          <span className={styles.itemType}>
            {displayNames.get(inst.type) ?? inst.type}
            {!inst.enabled && <span className={styles.offBadge}>off</span>}
          </span>
        </button>
      ))}
      <button
        type="button"
        className={styles.newBtn}
        onClick={onNewInstance}
      >
        + New instance
      </button>
    </aside>
  );
}
