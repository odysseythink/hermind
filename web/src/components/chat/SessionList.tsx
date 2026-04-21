import SessionItem from './SessionItem';
import type { SessionSummary } from '../../api/schemas';
import styles from './SessionList.module.css';

type Props = {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
};

export default function SessionList({ sessions, activeId, onSelect }: Props) {
  return (
    <ul className={styles.list}>
      {sessions.map((s) => (
        <SessionItem key={s.id} session={s} active={s.id === activeId} onClick={onSelect} />
      ))}
    </ul>
  );
}
