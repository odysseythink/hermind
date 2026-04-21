import { useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import StopButton from './StopButton';
import SlashMenu from './SlashMenu';

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
    <div
      style={{
        position: 'relative',
        display: 'flex',
        gap: '0.5rem',
        padding: '0.5rem',
        borderTop: '1px solid var(--border, #30363d)',
      }}
    >
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
        value={text}
        onChange={(e) => onChangeText(e.target.value)}
        onKeyDown={handleKey}
        placeholder={disabled ? t('chat.placeholderNoProvider') : t('chat.placeholder')}
        disabled={disabled}
        rows={3}
        style={{ flex: 1, resize: 'vertical', maxHeight: '16rem' }}
      />
      {streaming ? (
        <StopButton visible onClick={onStop} />
      ) : (
        <button type="button" onClick={onSend} disabled={disabled || !text.trim()}>
          {t('chat.send')}
        </button>
      )}
    </div>
  );
}
