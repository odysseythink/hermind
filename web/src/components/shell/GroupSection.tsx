import type { ReactNode } from 'react';
import styles from './GroupSection.module.css';
import { findGroup, type GroupId } from '../../shell/groups';

export interface GroupSectionProps {
  group: GroupId;
  expanded: boolean;
  active: boolean;
  dirty?: boolean;
  children?: ReactNode;
  onToggle: () => void;
  onSelectGroup: (id: GroupId) => void;
}

export default function GroupSection({
  group,
  expanded,
  active,
  dirty = false,
  children,
  onToggle,
  onSelectGroup,
}: GroupSectionProps) {
  const def = findGroup(group);
  const isGateway = group === 'gateway';
  return (
    <div className={styles.section}>
      <div className={styles.header}>
        <button
          type="button"
          aria-label={`toggle ${def.label}`}
          className={styles.toggle}
          onClick={onToggle}
        >
          {expanded ? '▾' : '▸'}
        </button>
        <button
          type="button"
          className={`${styles.label} ${active ? styles.active : ''}`}
          onClick={() => onSelectGroup(group)}
        >
          {def.label}
        </button>
        {dirty && <span className={styles.dirtyDot} title="Unsaved changes" />}
      </div>
      {expanded && (
        <div className={styles.children}>
          {isGateway
            ? children
            : <div className={styles.comingSoon}>Coming soon — stage {def.plannedStage}</div>}
        </div>
      )}
    </div>
  );
}
