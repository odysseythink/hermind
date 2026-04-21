import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SessionSummary } from '../../api/schemas';
import styles from './SessionItem.module.css';

const TITLE_MAX_RUNES = 200;

function runeLen(s: string): number {
  return Array.from(s).length;
}

type Props = {
  session: SessionSummary;
  active: boolean;
  onSelect: (id: string) => void;
  onRename: (id: string, title: string) => Promise<void> | void;
};

export default function SessionItem({ session, active, onSelect, onRename }: Props) {
  const { t } = useTranslation('ui');
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(session.title ?? '');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  function commit() {
    const trimmed = draft.trim();
    if (trimmed === '' || runeLen(trimmed) > TITLE_MAX_RUNES) {
      // stay in editing; force the user to fix or Esc
      inputRef.current?.focus();
      return;
    }
    if (trimmed === (session.title ?? '')) {
      setEditing(false);
      return;
    }
    Promise.resolve(onRename(session.id, trimmed))
      .then(() => setEditing(false))
      .catch(() => {
        setDraft(session.title ?? '');
        setEditing(false);
      });
  }

  function cancel() {
    setDraft(session.title ?? '');
    setEditing(false);
  }

  const displayTitle =
    session.title && session.title.length > 0
      ? session.title
      : t('chat.untitled');

  return (
    <li className={styles.item}>
      <button
        type="button"
        className={`${styles.btn} ${active ? styles.active : ''}`}
        aria-pressed={active}
        onClick={() => !editing && onSelect(session.id)}
      >
        {editing ? (
          <input
            ref={inputRef}
            type="text"
            className={styles.editInput}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                commit();
              } else if (e.key === 'Escape') {
                e.preventDefault();
                cancel();
              }
            }}
            onBlur={commit}
            onClick={(e) => e.stopPropagation()}
          />
        ) : (
          <span
            className={styles.title}
            title={t('chat.sidebar.doubleClickToRename')}
            onDoubleClick={(e) => {
              e.stopPropagation();
              setDraft(session.title ?? '');
              setEditing(true);
            }}
          >
            {displayTitle}
          </span>
        )}
        <span className={styles.sourceBadge}>
          {(session.source ?? '').toUpperCase()}
        </span>
      </button>
    </li>
  );
}
