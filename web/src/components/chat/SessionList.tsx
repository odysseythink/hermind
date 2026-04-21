import SessionItem from './SessionItem';
import type { SessionSummary as StateSummary } from '../../state/chat';
import type { SessionSummary as ApiSummary } from '../../api/schemas';

type Props = {
  sessions: Array<StateSummary | ApiSummary>;
  activeId: string | null;
  onSelect: (id: string) => void;
};

function normalize(s: StateSummary | ApiSummary): StateSummary {
  return {
    id: s.id,
    title: s.title ?? '',
    updatedAt:
      'updatedAt' in s
        ? s.updatedAt
        : typeof s.updated_at === 'number'
          ? s.updated_at
          : Date.now(),
  };
}

export default function SessionList({ sessions, activeId, onSelect }: Props) {
  return (
    <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
      {sessions.map((raw) => {
        const s = normalize(raw);
        return <SessionItem key={s.id} session={s} active={s.id === activeId} onClick={onSelect} />;
      })}
    </ul>
  );
}
