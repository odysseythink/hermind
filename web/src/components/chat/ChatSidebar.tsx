import NewChatButton from './NewChatButton';
import SessionList from './SessionList';
import type { SessionSummary as ApiSummary } from '../../api/schemas';
import styles from './ChatSidebar.module.css';

type Props = {
  sessions: ApiSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onRename: (id: string, title: string) => Promise<void> | void;
};

export default function ChatSidebar({ sessions, activeId, onSelect, onNew, onRename }: Props) {
  return (
    <aside className={styles.sidebar}>
      <NewChatButton onClick={onNew} />
      <SessionList
        sessions={sessions}
        activeId={activeId}
        onSelect={onSelect}
        onRename={onRename}
      />
    </aside>
  );
}
