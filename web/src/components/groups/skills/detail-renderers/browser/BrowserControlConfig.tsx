import { useState, useCallback } from 'react';
import pageStyles from '../../SkillToolsConfigPage.module.css';
import styles from './BrowserControlConfig.module.css';
import Switch from '../../../../fields/Switch';
import type { ToolDetailProps } from '../types';

type ConnectionStatus =
  | { state: 'unknown' }
  | { state: 'checking' }
  | { state: 'connected'; version: string }
  | { state: 'error'; message: string };

export default function BrowserControlConfig({
  name,
  description,
  toolset,
  enabled,
  onToggle,
}: ToolDetailProps) {
  const [status, setStatus] = useState<ConnectionStatus>({ state: 'unknown' });

  const handleTestConnection = useCallback(async () => {
    setStatus({ state: 'checking' });
    try {
      const resp = await fetch('/api/browser-extension/check');
      const data = (await resp.json()) as {
        connected?: boolean;
        version?: string;
        error?: string;
      };
      if (data.connected) {
        setStatus({ state: 'connected', version: data.version || 'unknown' });
      } else {
        setStatus({ state: 'error', message: data.error || 'Extension not responding' });
      }
    } catch (err) {
      setStatus({
        state: 'error',
        message: err instanceof Error ? err.message : 'Network error',
      });
    }
  }, []);

  const renderStatusCard = () => {
    switch (status.state) {
      case 'connected':
        return (
          <div className={`${styles.statusCard} ${styles.statusConnected}`} data-testid="status-connected">
            <div className={styles.statusIcon}>🟢</div>
            <div>
              <div className={styles.statusTitle}>已连接</div>
              <div className={styles.statusMeta}>Extension version {status.version}</div>
            </div>
          </div>
        );
      case 'error':
        return (
          <div className={`${styles.statusCard} ${styles.statusError}`} data-testid="status-error">
            <div className={styles.statusIcon}>🔴</div>
            <div>
              <div className={styles.statusTitle}>未连接</div>
              <div className={styles.statusMeta}>{status.message}</div>
            </div>
          </div>
        );
      case 'checking':
        return (
          <div className={`${styles.statusCard} ${styles.statusChecking}`} data-testid="status-checking">
            <div className={styles.statusIcon}>⏳</div>
            <div>
              <div className={styles.statusTitle}>检测中...</div>
            </div>
          </div>
        );
      default:
        return (
          <div className={`${styles.statusCard} ${styles.statusUnknown}`} data-testid="status-unknown">
            <div className={styles.statusIcon}>⚪</div>
            <div>
              <div className={styles.statusTitle}>状态未知</div>
              <div className={styles.statusMeta}>点击「测试连接」检查扩展状态</div>
            </div>
          </div>
        );
    }
  };

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          <span className={pageStyles.detailEmoji}>🌐</span>
          {name}
          {toolset && (
            <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>
      {description && <div className={pageStyles.detailDesc}>{description}</div>}

      {renderStatusCard()}

      <div className={styles.actionBar}>
        <button
          type="button"
          className={styles.primaryBtn}
          onClick={handleTestConnection}
          disabled={status.state === 'checking'}
          data-testid="test-connection-btn"
        >
          {status.state === 'checking' ? '检测中...' : '测试连接'}
        </button>
      </div>

      {(status.state === 'error' || status.state === 'unknown') && (
        <div className={styles.installGuide} data-testid="install-guide">
          <h4>安装浏览器扩展</h4>
          <ol>
            <li>下载并解压浏览器扩展包</li>
            <li>打开 Chrome 扩展管理页面（chrome://extensions/）</li>
            <li>开启「开发者模式」</li>
            <li>点击「加载已解压的扩展程序」，选择解压后的文件夹</li>
          </ol>
        </div>
      )}
    </div>
  );
}
