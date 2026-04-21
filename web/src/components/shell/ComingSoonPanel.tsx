import styles from './ComingSoonPanel.module.css';
import type { Config } from '../../api/schemas';
import { findGroup, type GroupId } from '../../shell/groups';
import { summaryFor } from '../../shell/summaries';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../i18n/useDescriptorT';

export interface ComingSoonPanelProps {
  group: GroupId;
  config: Config;
}

export default function ComingSoonPanel({ group, config }: ComingSoonPanelProps) {
  const def = findGroup(group);
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const groupLabel = dt.groupLabel(def.id, def.label);
  return (
    <section className={styles.panel} aria-label={t('empty.comingSoon', { group: groupLabel })}>
      <div className={styles.label}>{groupLabel}</div>
      <h2 className={styles.title}>{t('empty.comingSoon', { group: groupLabel })}</h2>
      <span className={styles.stage}>{t('empty.plannedStage', { stage: def.plannedStage })}</span>
      <p className={styles.desc}>{t(`group.${def.id}.description`)}</p>

      <div className={styles.label}>{t('empty.thisSectionCovers')}</div>
      <ul className={styles.bullets}>
        {def.bullets.map(b => (
          <li key={b}>{b}</li>
        ))}
      </ul>

      <div className={styles.previewLabel}>{t('empty.currentConfig')}</div>
      <div className={styles.preview}>{summaryFor(group, config)}</div>

      <div className={styles.escape}>
        {t('empty.escape', { command: '' })}{' '}
        <code>hermind config --web</code>
      </div>
    </section>
  );
}
