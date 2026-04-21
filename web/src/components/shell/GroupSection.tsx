import type { ReactNode } from 'react';
import styles from './GroupSection.module.css';
import { findGroup, type GroupId } from '../../shell/groups';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../i18n/useDescriptorT';

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
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const label = dt.groupLabel(def.id, def.label);
  return (
    <div className={styles.section}>
      <div className={styles.header}>
        <button
          type="button"
          aria-label={t('sidebar.toggle', { group: label })}
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
          {label}
        </button>
        {dirty && <span className={styles.dirtyDot} title={t('empty.unsaved')} />}
      </div>
      {expanded && (
        <div className={styles.children}>
          {children}
        </div>
      )}
    </div>
  );
}
