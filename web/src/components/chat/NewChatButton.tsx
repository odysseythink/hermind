import { useTranslation } from 'react-i18next';
import styles from './NewChatButton.module.css';

type Props = { onClick: () => void };

export default function NewChatButton({ onClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <button type="button" className={styles.btn} onClick={onClick}>
      {t('chat.newConversation')}
    </button>
  );
}
