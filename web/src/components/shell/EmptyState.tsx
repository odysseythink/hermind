import styles from './EmptyState.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';

export interface EmptyStateProps {
  onSelectGroup: (id: GroupId) => void;
}

export default function EmptyState({ onSelectGroup }: EmptyStateProps) {
  return (
    <section className={styles.empty}>
      <h2 className={styles.title}>Select a configuration section</h2>
      <div className={styles.grid}>
        {GROUPS.map(g => (
          <button
            key={g.id}
            type="button"
            className={styles.card}
            onClick={() => onSelectGroup(g.id)}
          >
            <div className={styles.cardLabel}>{g.label}</div>
            <div className={styles.cardDesc}>{g.description}</div>
            <span className={styles.cardStage}>
              {g.plannedStage === 'done' ? 'available' : `stage ${g.plannedStage}`}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}
