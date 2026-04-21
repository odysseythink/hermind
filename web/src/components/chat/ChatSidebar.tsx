import NewChatButton from './NewChatButton';
import SessionList from './SessionList';
import type { SessionSummary as ApiSummary } from '../../api/schemas';

type Props = {
  sessions: ApiSummary[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
};

export default function ChatSidebar({ sessions, activeId, onSelect, onNew }: Props) {
  return (
    <aside style={{ width: '14rem', borderRight: '1px solid var(--border, #30363d)', padding: '0.5rem' }}>
      <NewChatButton onClick={onNew} />
      <SessionList sessions={sessions} activeId={activeId} onSelect={onSelect} />
    </aside>
  );
}
