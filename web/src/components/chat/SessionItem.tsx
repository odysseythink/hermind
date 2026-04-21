import { useTranslation } from 'react-i18next';
import type { SessionSummary } from '../../state/chat';

type Props = {
  session: SessionSummary;
  active: boolean;
  onClick: (id: string) => void;
};

export default function SessionItem({ session, active, onClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <li>
      <button
        type="button"
        aria-pressed={active}
        onClick={() => onClick(session.id)}
      >
        {session.title || t('chat.untitledConversation')}
      </button>
    </li>
  );
}
