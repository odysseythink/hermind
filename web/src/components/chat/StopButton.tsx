import { useTranslation } from 'react-i18next';
import styles from './StopButton.module.css';

type Props = { visible: boolean; onClick: () => void };

export default function StopButton({ visible, onClick }: Props) {
  const { t } = useTranslation('ui');
  if (!visible) return null;
  return (
    <button type="button" className={styles.btn} onClick={onClick}>
      ■ {t('chat.stop')}
    </button>
  );
}
