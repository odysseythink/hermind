import { useTranslation } from 'react-i18next';
import type { SessionSummary } from '../../state/chat';
import styles from './SessionItem.module.css';

type Props = {
  session: SessionSummary;
  active: boolean;
  onClick: (id: string) => void;
};

export default function SessionItem({ session, active, onClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <li className={styles.item}>
      <button
        type="button"
        className={styles.btn}
        aria-pressed={active}
        onClick={() => onClick(session.id)}
      >
        {session.title || t('chat.untitledConversation')}
      </button>
    </li>
  );
}
