import styles from './ComingSoonPanel.module.css';
import type { Config } from '../../api/schemas';
import { findGroup, type GroupId } from '../../shell/groups';
import { summaryFor } from '../../shell/summaries';

export interface ComingSoonPanelProps {
  group: GroupId;
  config: Config;
}

export default function ComingSoonPanel({ group, config }: ComingSoonPanelProps) {
  const def = findGroup(group);
  return (
    <section className={styles.panel} aria-label={`${def.label} — coming soon`}>
      <div className={styles.label}>{def.label}</div>
      <h2 className={styles.title}>{def.label} — coming soon</h2>
      <span className={styles.stage}>Planned for stage {def.plannedStage}</span>
      <p className={styles.desc}>{def.description}</p>

      <div className={styles.label}>This section will cover</div>
      <ul className={styles.bullets}>
        {def.bullets.map(b => (
          <li key={b}>{b}</li>
        ))}
      </ul>

      <div className={styles.previewLabel}>Current config (read-only preview)</div>
      <div className={styles.preview}>{summaryFor(group, config)}</div>

      <div className={styles.escape}>
        Need to edit this now? Run <code>hermind config --web</code> for the legacy editor.
      </div>
    </section>
  );
}
