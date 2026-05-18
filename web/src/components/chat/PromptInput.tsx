import { useRef, useCallback, useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './PromptInput.module.css';
import AttachmentUploader from './AttachmentUploader';
import MentionButton from './MentionButton';
import ToolsButton from './ToolsButton';
import ToolsMenu from './ToolsMenu';
import type { Attachment } from '../../state/chat';

interface Props {
  text: string;
  onTextChange: (text: string) => void;
  onSubmit: () => void;
  disabled?: boolean;
  attachments?: Attachment[];
  onAttachmentsAdd?: (attachments: Attachment[]) => void;
  onAttachmentRemove?: (id: string) => void;
  suggestions?: string[];
}

export default function PromptInput({
  text,
  onTextChange,
  onSubmit,
  disabled,
  attachments,
  onAttachmentsAdd,
  onAttachmentRemove,
  suggestions = [],
}: Props) {
  const { t } = useTranslation('ui');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [toolsOpen, setToolsOpen] = useState(false);

  useEffect(() => {
    if (!text) {
      const el = textareaRef.current;
      if (el) {
        el.style.height = 'auto';
      }
    }
  }, [text]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        onSubmit();
      }
    },
    [onSubmit],
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      onTextChange(e.target.value);
      const el = e.target;
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
    },
    [onTextChange],
  );

  const handleMention = () => {
    const newText = text + '@';
    onTextChange(newText);
    textareaRef.current?.focus();
  };

  const handleToolsSelect = (selected: string, sendNow?: boolean) => {
    onTextChange(selected);
    if (sendNow) {
      onSubmit();
    } else {
      textareaRef.current?.focus();
    }
  };

  const placeholder = disabled ? t('chat.placeholderNoProvider') : t('chat.placeholder');

  return (
    <div className={styles.inputWrapper}>
      <textarea
        ref={textareaRef}
        className={styles.textarea}
        value={text}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        rows={1}
        disabled={disabled}
      />

      <div className={styles.buttonRow}>
        <div className={styles.leftButtons}>
          <AttachmentUploader onAttachmentsAdd={onAttachmentsAdd ?? (() => {})} />
          <MentionButton onClick={handleMention} disabled={disabled} />
          <div className={styles.toolsWrapper}>
            <ToolsButton
              onClick={() => setToolsOpen((v) => !v)}
              active={toolsOpen}
              disabled={disabled}
            />
            <ToolsMenu
              visible={toolsOpen}
              onClose={() => setToolsOpen(false)}
              onSelect={handleToolsSelect}
              suggestions={suggestions}
            />
          </div>
        </div>
        <button
          className={styles.sendBtn}
          onClick={onSubmit}
          disabled={disabled || !text.trim()}
          aria-label={t('chat.send')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="22" y1="2" x2="11" y2="13" />
            <polygon points="22 2 15 22 11 13 2 9" />
          </svg>
        </button>
      </div>

      {attachments && attachments.length > 0 && (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '2px',
            maxHeight: '80px',
            overflowY: 'auto',
            marginTop: '8px',
          }}
        >
          {attachments.map((att) => (
            <div
              key={att.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                fontSize: '12px',
                color: 'var(--muted)',
              }}
            >
              <span
                style={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: '150px',
                }}
              >
                {att.name}
              </span>
              {onAttachmentRemove && (
                <button
                  onClick={() => onAttachmentRemove(att.id)}
                  style={{
                    background: 'none',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--muted)',
                    fontSize: '14px',
                    lineHeight: 1,
                    padding: 0,
                  }}
                  aria-label={`Remove ${att.name}`}
                >
                  ×
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
