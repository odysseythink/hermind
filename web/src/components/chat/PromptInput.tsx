import { useRef, useCallback } from 'react';
import styles from './PromptInput.module.css';

interface Props {
  text: string;
  onTextChange: (text: string) => void;
  onSubmit: () => void;
  disabled?: boolean;
}

export default function PromptInput({ text, onTextChange, onSubmit, disabled }: Props) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

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
      // Auto-resize
      const el = e.target;
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
    },
    [onTextChange],
  );

  return (
    <div className={styles.inputBar}>
      <textarea
        ref={textareaRef}
        className={styles.textarea}
        value={text}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder="Type a message..."
        rows={1}
        disabled={disabled}
      />
      <button
        className={styles.sendBtn}
        onClick={onSubmit}
        disabled={disabled || !text.trim()}
        aria-label="Send message"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="22" y1="2" x2="11" y2="13" />
          <polygon points="22 2 15 22 11 13 2 9" />
        </svg>
      </button>
    </div>
  );
}
