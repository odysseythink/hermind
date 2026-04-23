import styles from './GatewaySidebar.module.css';
import { useTranslation } from 'react-i18next';

export interface GatewaySidebarProps {
  activeSubKey: string | null;
  platformInstances: Array<{ key: string; type: string; enabled: boolean }>;
  dirtyPlatformKeys: Set<string>;
  onSelectPlatform: (key: string) => void;
  onAddPlatform: () => void;
}

export default function GatewaySidebar({
  activeSubKey,
  platformInstances,
  dirtyPlatformKeys,
  onSelectPlatform,
  onAddPlatform,
}: GatewaySidebarProps) {
  const { t } = useTranslation('ui');

  return (
    <div className={styles.sidebar}>
      <div className={styles.groupHeader}>{t('sidebar.imChannels')}</div>
      {platformInstances.length === 0 && (
        <div className={styles.empty}>{t('sidebar.noPlatforms')}</div>
      )}
      {platformInstances.map(inst => {
        const active = activeSubKey === `gateway:${inst.key}`;
        return (
          <button
            key={inst.key}
            type="button"
            className={`${styles.platformRow} ${active ? styles.active : ''} ${!inst.enabled ? styles.disabled : ''}`}
            onClick={() => onSelectPlatform(inst.key)}
          >
            <span className={styles.platformRowInner}>
              <span className={styles.platformName}>{inst.key}</span>
              {dirtyPlatformKeys.has(inst.key) && (
                <span className={styles.dirtyDot} title={t('empty.unsaved')} />
              )}
            </span>
            <span className={styles.platformType}>{inst.type}</span>
          </button>
        );
      })}
      <button type="button" className={styles.newBtn} onClick={onAddPlatform}>
        {t('sidebar.addPlatform')}
      </button>
    </div>
  );
}
