import styles from './AdvancedSidebar.module.css';

export interface AdvancedSidebarProps {
  activeSubKey: string | null;
  onSelectScalar: (key: string) => void;
  // Cron
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
  cronJobs,
  dirtyCronIndices,
  activeCronIndex,
  onSelectCron,
  onAddCronJob,
  onMoveCron,
}: AdvancedSidebarProps) {
  return (
    <div className={styles.sidebar}>
      <button
        type="button"
        className={`${styles.scalarRow} ${activeSubKey === 'browser' ? styles.active : ''}`}
        onClick={() => onSelectScalar('browser')}
      >
        Browser
      </button>

      <div className={styles.groupHeader}>Cron jobs</div>
      {cronJobs.length === 0 && (
        <div className={styles.empty}>No cron jobs configured.</div>
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
                <span className={styles.cronName}>{job.name || '(unnamed)'}</span>
                {dirtyCronIndices.has(i) && (
                  <span className={styles.dirtyDot} title="Unsaved changes" />
                )}
              </span>
              <span className={styles.cronSchedule}>{job.schedule}</span>
            </button>
            <div className={styles.cronMoveBtns}>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move up"
                disabled={atTop}
                onClick={() => onMoveCron(i, 'up')}
              >
                ↑
              </button>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label="Move down"
                disabled={atBottom}
                onClick={() => onMoveCron(i, 'down')}
              >
                ↓
              </button>
            </div>
          </div>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddCronJob}>
        + Add cron job
      </button>
    </div>
  );
}
