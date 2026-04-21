import styles from './AdvancedSidebar.module.css';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../../i18n/useDescriptorT';

export interface AdvancedSidebarProps {
  activeSubKey: string | null;
  onSelectScalar: (key: string) => void;
  mcpInstances: Array<{ key: string; command: string; enabled: boolean }>;
  dirtyMcpKeys: Set<string>;
  onSelectMcp: (key: string) => void;
  onAddMcpServer: () => void;
  cronJobs: Array<{ name: string; schedule: string }>;
  dirtyCronIndices: Set<number>;
  activeCronIndex: number | null;
  onSelectCron: (index: number) => void;
  onAddCronJob: () => void;
  onMoveCron: (index: number, direction: 'up' | 'down') => void;
}

export default function AdvancedSidebar({
  activeSubKey,
  onSelectScalar,
  mcpInstances,
  dirtyMcpKeys,
  onSelectMcp,
  onAddMcpServer,
  cronJobs,
  dirtyCronIndices,
  activeCronIndex,
  onSelectCron,
  onAddCronJob,
  onMoveCron,
}: AdvancedSidebarProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  return (
    <div className={styles.sidebar}>
      <button
        type="button"
        className={`${styles.scalarRow} ${activeSubKey === 'browser' ? styles.active : ''}`}
        onClick={() => onSelectScalar('browser')}
      >
        {dt.sectionLabel('browser', t('sidebar.browser'))}
      </button>

      <div className={styles.groupHeader}>{t('sidebar.mcpServers')}</div>
      {mcpInstances.length === 0 && (
        <div className={styles.empty}>{t('sidebar.noMcp')}</div>
      )}
      {mcpInstances.map(inst => {
        const active = activeSubKey === `mcp:${inst.key}`;
        return (
          <button
            key={inst.key}
            type="button"
            className={`${styles.mcpRow} ${active ? styles.active : ''} ${!inst.enabled ? styles.disabled : ''}`}
            onClick={() => onSelectMcp(inst.key)}
          >
            <span className={styles.mcpRowInner}>
              <span className={styles.mcpName}>{inst.key}</span>
              {dirtyMcpKeys.has(inst.key) && (
                <span className={styles.dirtyDot} title={t('empty.unsaved')} />
              )}
            </span>
            <span className={styles.mcpCommand}>{inst.command || t('field.noCommand')}</span>
          </button>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddMcpServer}>
        {t('sidebar.addMcp')}
      </button>

      <div className={styles.groupHeader}>{t('sidebar.cronJobs')}</div>
      {cronJobs.length === 0 && (
        <div className={styles.empty}>{t('sidebar.noCron')}</div>
      )}
      {cronJobs.map((job, i) => {
        const active = i === activeCronIndex;
        const atTop = i === 0;
        const atBottom = i === cronJobs.length - 1;
        return (
          <div
            key={i}
            className={`${styles.cronRow} ${active ? styles.active : ''}`}
          >
            <button
              type="button"
              className={styles.cronBody}
              onClick={() => onSelectCron(i)}
            >
              <span className={styles.cronRowInner}>
                <span className={styles.posBadge}>#{i + 1}</span>
                <span className={styles.cronName}>{job.name || t('field.unnamed')}</span>
                {dirtyCronIndices.has(i) && (
                  <span className={styles.dirtyDot} title={t('empty.unsaved')} />
                )}
              </span>
              <span className={styles.cronSchedule}>{job.schedule}</span>
            </button>
            <div className={styles.cronMoveBtns}>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label={t('sidebar.moveUp')}
                disabled={atTop}
                onClick={() => onMoveCron(i, 'up')}
              >↑</button>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label={t('sidebar.moveDown')}
                disabled={atBottom}
                onClick={() => onMoveCron(i, 'down')}
              >↓</button>
            </div>
          </div>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddCronJob}>
        {t('sidebar.addCron')}
      </button>
    </div>
  );
}
