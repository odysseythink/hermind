import { useTranslation } from 'react-i18next';
import SettingsButton from './SettingsButton';
import styles from './ConversationHeader.module.css';

type Props = {
  title: string;
  instanceRoot: string;
  onOpenSettings: () => void;
  settingsDisabled?: boolean;
};

export default function ConversationHeader({
  title,
  instanceRoot,
  onOpenSettings,
  settingsDisabled,
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
      <h2 className={styles.title}>{title}</h2>
      <SettingsButton
        onClick={onOpenSettings}
        disabled={settingsDisabled}
        ariaLabel={t('chat.settings.title')}
      />
    </header>
  );
}
