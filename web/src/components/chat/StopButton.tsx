import { useTranslation } from 'react-i18next';

type Props = { visible: boolean; onClick: () => void };

export default function StopButton({ visible, onClick }: Props) {
  const { t } = useTranslation('ui');
  if (!visible) return null;
  return (
    <button type="button" onClick={onClick}>
      ■ {t('chat.stop')}
    </button>
  );
}
