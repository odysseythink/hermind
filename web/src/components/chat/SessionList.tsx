import SessionItem from './SessionItem';
import type { SessionSummary } from '../../api/schemas';
import styles from './SessionList.module.css';

type Props = {
  sessions: SessionSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onRename: (id: string, title: string) => Promise<void> | void;
};

export default function SessionList({ sessions, activeId, onSelect, onRename }: Props) {
  return (
    <ul className={styles.list}>
      {sessions.map((s) => (
        <SessionItem
          key={s.id}
          session={s}
          active={s.id === activeId}
          onSelect={onSelect}
          onRename={onRename}
        />
      ))}
    </ul>
  );
}
