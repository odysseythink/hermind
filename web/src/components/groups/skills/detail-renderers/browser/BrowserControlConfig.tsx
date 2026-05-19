import { useState, useCallback } from 'react';
import pageStyles from '../../SkillToolsConfigPage.module.css';
import styles from './BrowserControlConfig.module.css';
import Switch from '../../../../fields/Switch';
import type { ToolDetailProps } from '../types';

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

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
  config,
  onSectionField,
}: ToolDetailProps) {
  const [status, setStatus] = useState<ConnectionStatus>({ state: 'unknown' });

  const apiKey = asString(
    (config?.browser_extension as Record<string, unknown> | undefined)?.api_key,
  );

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

  const handleKeyChange = (value: string) => {
    onSectionField('browser_extension', 'api_key', value);
  };

  const handleCopyKey = async () => {
    if (!apiKey) return;
    try {
      await navigator.clipboard.writeText(apiKey);
    } catch {
      // ignore
    }
  };

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

      <div className={pageStyles.configSection}>
        <h3>认证</h3>
        <div className={styles.configRow}>
          <div>
            <div className={styles.label}>Extension API Key</div>
            <div className={styles.help}>浏览器扩展通过此密钥与 Hermind 建立安全通信</div>
          </div>
          <div className={styles.keyInputRow}>
            <input
              type="password"
              className={styles.keyInput}
              value={apiKey}
              onChange={(e) => handleKeyChange(e.currentTarget.value)}
              placeholder="输入或生成 API Key"
              aria-label="Extension API Key"
              data-testid="api-key-input"
            />
            <button type="button" className={styles.iconBtn} onClick={handleCopyKey} title="复制">
              📋
            </button>
          </div>
        </div>
      </div>

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
            <li>在扩展设置中粘贴上方的 API Key</li>
          </ol>
        </div>
      )}
    </div>
  );
}
