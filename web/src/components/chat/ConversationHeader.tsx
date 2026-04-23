import { useTranslation } from 'react-i18next';
import styles from './ConversationHeader.module.css';

type Props = {
  instanceRoot: string;
  modelOptions: string[];
  selectedModel: string;
  onSelectModel: (model: string) => void;
  onStop: () => void;
  streaming: boolean;
};

export default function ConversationHeader({
  instanceRoot,
  modelOptions,
  selectedModel,
  onSelectModel,
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
      <select
        className={styles.modelSelect}
        value={selectedModel}
        onChange={(e) => onSelectModel(e.target.value)}
        aria-label={t('chat.modelDropdown', { defaultValue: 'Model' })}
      >
        {modelOptions.length === 0 && <option value="">(no models)</option>}
        {modelOptions.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
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
