import { useState } from 'react';
import styles from './SourcesSidebar.module.css';
import type { Source } from '../../state/chat';

interface Props {
  sources: Source[];
}

export default function SourcesSidebar({ sources }: Props) {
  const [isOpen, setIsOpen] = useState(false);

  if (!isOpen) {
    return (
      <button
        className={styles.toggleBtn}
        onClick={() => setIsOpen(true)}
        aria-label="Open sources"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 19.5A2.5 2.5 0 016.5 17H20" />
          <path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z" />
        </svg>
        {sources.length > 0 && <span className={styles.badge}>{sources.length}</span>}
      </button>
    );
  }

  return (
    <aside className={styles.sidebar}>
      <div className={styles.header}>
        <h3 className={styles.title}>Sources</h3>
        <button className={styles.closeBtn} onClick={() => setIsOpen(false)} aria-label="Close sources">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>
      <div className={styles.sourceList}>
        {sources.length === 0 ? (
          <p className={styles.empty}>No sources available</p>
        ) : (
          sources.map((source) => (
            <div key={source.id} className={styles.sourceItem}>
              <div className={styles.sourceTitle}>{source.title}</div>
              <div className={styles.sourceText}>{source.text}</div>
            </div>
          ))
        )}
      </div>
    </aside>
  );
}
