import { useState } from 'react';
import styles from './ModelsSidebar.module.css';
import { useTranslation } from 'react-i18next';
import { useDescriptorT } from '../../../i18n/useDescriptorT';

export interface ModelsSidebarProps {
  instances: Array<{ key: string; type: string }>;
  activeSubKey: string | null;
  dirtyKeys: Set<string>;
  onSelectScalar: (key: string) => void;
  onSelectInstance: (key: string) => void;
  onNewProvider: () => void;
  // Stage 4c additions
  fallbackProviders: Array<{ provider: string }>;
  dirtyFallbackIndices: Set<number>;
  activeFallbackIndex: number | null;
  onSelectFallback: (index: number) => void;
  onAddFallback: () => void;
  onMoveFallback: (index: number, direction: 'up' | 'down') => void;
  // Stage 4f addition
  onReorderFallback: (from: number, to: number) => void;
}

export default function ModelsSidebar({
  instances,
  activeSubKey,
  dirtyKeys,
  onSelectScalar,
  onSelectInstance,
  onNewProvider,
  fallbackProviders,
  dirtyFallbackIndices,
  activeFallbackIndex,
  onSelectFallback,
  onAddFallback,
  onMoveFallback,
  onReorderFallback,
}: ModelsSidebarProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const [dragOver, setDragOver] = useState<number | null>(null);
  return (
    <div className={styles.sidebar}>
      <button
        type="button"
        className={`${styles.scalarRow} ${activeSubKey === 'model' ? styles.active : ''}`}
        onClick={() => onSelectScalar('model')}
      >
        {dt.sectionLabel('model', t('sidebar.defaultModel'))}
      </button>
      <div className={styles.groupHeader}>{t('sidebar.providers')}</div>
      {instances.length === 0 && (
        <div className={styles.empty}>{t('sidebar.noProviders')}</div>
      )}
      {instances.map(inst => (
        <button
          key={inst.key}
          type="button"
          className={`${styles.item} ${inst.key === activeSubKey ? styles.active : ''}`}
          onClick={() => onSelectInstance(inst.key)}
        >
          <span className={styles.itemRow}>
            <span className={styles.itemKey}>{inst.key}</span>
            {dirtyKeys.has(inst.key) && (
              <span className={styles.dirtyDot} title={t('empty.unsaved')} />
            )}
          </span>
          <span className={styles.itemType}>{inst.type}</span>
        </button>
      ))}
      <button type="button" className={styles.newBtn} onClick={onNewProvider}>
        {t('sidebar.newProvider')}
      </button>
      <div className={styles.groupHeader}>{t('sidebar.fallbackProviders')}</div>
      {fallbackProviders.length === 0 && (
        <div className={styles.empty}>{t('sidebar.noFallbacks')}</div>
      )}
      {fallbackProviders.map((fb, i) => {
        const active = i === activeFallbackIndex;
        const atTop = i === 0;
        const atBottom = i === fallbackProviders.length - 1;
        const isDragging = dragFrom === i;
        const isDragOver = dragOver === i && dragFrom !== null && dragFrom !== i;
        return (
          <div
            key={i}
            data-fallback-row
            draggable
            onDragStart={e => {
              setDragFrom(i);
              e.dataTransfer.effectAllowed = 'move';
              try { e.dataTransfer.setData('text/plain', String(i)); } catch { /* Firefox */ }
            }}
            onDragOver={e => {
              if (dragFrom === null) return;
              e.preventDefault();
              e.dataTransfer.dropEffect = 'move';
              if (dragOver !== i) setDragOver(i);
            }}
            onDragLeave={() => {
              if (dragOver === i) setDragOver(null);
            }}
            onDrop={e => {
              e.preventDefault();
              if (dragFrom !== null && dragFrom !== i) {
                onReorderFallback(dragFrom, i);
              }
              setDragFrom(null);
              setDragOver(null);
            }}
            onDragEnd={() => {
              setDragFrom(null);
              setDragOver(null);
            }}
            className={`${styles.fallbackRow} ${active ? styles.active : ''} ${isDragging ? styles.dragging : ''} ${isDragOver ? styles.dragOver : ''}`}
          >
            <button
              type="button"
              className={styles.fallbackBody}
              onClick={() => onSelectFallback(i)}
            >
              <span className={styles.fallbackRowInner}>
                <span className={styles.posBadge}>#{i + 1}</span>
                <span className={styles.fallbackType}>{fb.provider}</span>
                {dirtyFallbackIndices.has(i) && (
                  <span className={styles.dirtyDot} title={t('empty.unsaved')} />
                )}
              </span>
            </button>
            <div className={styles.fallbackMoveBtns}>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label={t('sidebar.moveUp')}
                disabled={atTop}
                onClick={() => onMoveFallback(i, 'up')}
              >
                ↑
              </button>
              <button
                type="button"
                className={styles.moveBtn}
                aria-label={t('sidebar.moveDown')}
                disabled={atBottom}
                onClick={() => onMoveFallback(i, 'down')}
              >
                ↓
              </button>
            </div>
          </div>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddFallback}>
        {t('sidebar.addFallback')}
      </button>
    </div>
  );
}
