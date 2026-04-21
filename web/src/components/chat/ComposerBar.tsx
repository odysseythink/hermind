import { useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import StopButton from './StopButton';
import SlashMenu from './SlashMenu';
import styles from './ComposerBar.module.css';

type Props = {
  text: string;
  onChangeText: (v: string) => void;
  onSend: () => void;
  onStop: () => void;
  disabled: boolean;
  streaming: boolean;
  onSlashCommand?: (cmd: string) => void;
};

export default function ComposerBar({
  text, onChangeText, onSend, onStop, disabled, streaming, onSlashCommand,
}: Props) {
  const { t } = useTranslation('ui');
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const handleKey = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey && !text.startsWith('/')) {
        e.preventDefault();
        if (text.trim()) onSend();
      }
    },
    [text, onSend],
  );

  const slashOpen = text.startsWith('/') && !!onSlashCommand;

  return (
    <div className={styles.bar}>
      {slashOpen && onSlashCommand && (
        <SlashMenu
          commands={[
            { id: 'new', label: 'new', run: () => onSlashCommand('new') },
            { id: 'clear', label: 'clear', run: () => onChangeText('') },
            { id: 'settings', label: 'settings', run: () => onSlashCommand('settings') },
            { id: 'model', label: 'model', run: () => onSlashCommand('model') },
          ]}
          onClose={() => onChangeText(text.replace(/^\/[^\s]*\s?/, ''))}
        />
      )}
      <textarea
        ref={inputRef}
        className={styles.textarea}
        value={text}
        onChange={(e) => onChangeText(e.target.value)}
        onKeyDown={handleKey}
        placeholder={disabled ? t('chat.placeholderNoProvider') : t('chat.placeholder')}
        disabled={disabled}
        rows={3}
      />
      {streaming ? (
        <StopButton visible onClick={onStop} />
      ) : (
        <button
          type="button"
          className={styles.sendBtn}
          onClick={onSend}
          disabled={disabled || !text.trim()}
        >
          {t('chat.send')}
        </button>
      )}
    </div>
  );
}
