import { useTranslation } from 'react-i18next';
import styles from './ConversationHeader.module.css';

type Props = {
  instanceRoot: string;
  onStop: () => void;
  streaming: boolean;
};

export default function ConversationHeader({
  instanceRoot,
  onStop,
  streaming,
}: Props) {
  const { t } = useTranslation('ui');
  return (
    <header className={styles.header}>
      {instanceRoot && (
        <span
          className={styles.instancePath}
          title={instanceRoot}
          dir="rtl"
          aria-label={t('chat.instance.label', { defaultValue: 'Instance' })}
        >
          {instanceRoot}
        </span>
      )}
      <div className={styles.spacer} />
      <button
        type="button"
        className={styles.stopBtn}
        disabled={!streaming}
        onClick={onStop}
        aria-label={t('chat.stop', { defaultValue: 'Stop' })}
      >
        {t('chat.stop', { defaultValue: 'Stop' })}
      </button>
    </header>
  );
}
