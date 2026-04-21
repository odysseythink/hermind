import styles from './SectionList.module.css';
import { findGroup, type GroupId } from '../../shell/groups';
import type { ConfigSection } from '../../api/schemas';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../i18n/useDescriptorT';

export interface SectionListProps {
  group: GroupId;
  sections: readonly ConfigSection[];
  activeSubKey: string | null;
  onSelect: (key: string) => void;
}

export default function SectionList({
  group,
  sections,
  activeSubKey,
  onSelect,
}: SectionListProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  if (sections.length === 0) {
    const def = findGroup(group);
    return (
      <div className={styles.comingSoon}>
        {t('empty.sectionComingSoon', { stage: def.plannedStage })}
      </div>
    );
  }
  return (
    <div className={styles.list}>
      {sections.map(s => {
        const active = activeSubKey === s.key;
        return (
          <button
            key={s.key}
            type="button"
            className={`${styles.row} ${active ? styles.active : ''}`}
            aria-current={active ? 'true' : undefined}
            onClick={() => onSelect(s.key)}
          >
            {dt.sectionLabel(s.key, s.label)}
          </button>
        );
      })}
    </div>
  );
}
