import styles from './ComingSoonPanel.module.css';
import type { Config } from '../../api/schemas';
import { findGroup, type GroupId } from '../../shell/groups';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../i18n/useDescriptorT';

export interface ComingSoonPanelProps {
  group: GroupId;
  config: Config;
}

export default function ComingSoonPanel({ group }: ComingSoonPanelProps) {
  const def = findGroup(group);
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const groupLabel = dt.groupLabel(def.id, def.label);
  return (
    <section className={styles.panel} aria-label={t('empty.comingSoon', { group: groupLabel })}>
      <div className={styles.label}>{groupLabel}</div>
      <h2 className={styles.title}>{t('empty.comingSoon', { group: groupLabel })}</h2>
      <p className={styles.desc}>{t(`group.${def.id}.description`)}</p>

      <div className={styles.label}>{t('empty.thisSectionCovers')}</div>
      <ul className={styles.bullets}>
        {def.bullets.map((b) => (
          <li key={b}>{b}</li>
        ))}
      </ul>
    </section>
  );
}
