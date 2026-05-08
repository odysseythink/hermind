import styles from './EmptyState.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../i18n/useDescriptorT';

export interface EmptyStateProps {
  onSelectGroup: (id: GroupId) => void;
}

export default function EmptyState({ onSelectGroup }: EmptyStateProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  return (
    <section className={styles.empty}>
      <h2 className={styles.title}>{t('empty.title')}</h2>
      <div className={styles.grid}>
        {GROUPS.map(g => (
          <button
            key={g.id}
            type="button"
            className={styles.card}
            onClick={() => onSelectGroup(g.id)}
          >
            <div className={styles.cardLabel}>{dt.groupLabel(g.id, g.label)}</div>
            <div className={styles.cardDesc}>{t(`group.${g.id}.description`)}</div>
            <span className={styles.cardStage}>
              {t('empty.available')}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}
