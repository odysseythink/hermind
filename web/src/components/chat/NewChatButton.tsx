import { useTranslation } from 'react-i18next';

type Props = { onClick: () => void };

export default function NewChatButton({ onClick }: Props) {
  const { t } = useTranslation('ui');
  return (
    <button type="button" onClick={onClick}>
      {t('chat.newConversation')}
    </button>
  );
}
