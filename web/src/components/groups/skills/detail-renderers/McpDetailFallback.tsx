import pageStyles from '../SkillToolsConfigPage.module.css';
import styles from './McpDetailFallback.module.css';
import Switch from '../../../fields/Switch';
import type { McpDetailProps } from './types';

export default function McpDetailFallback({
  key,
  command,
  enabled,
  onToggle,
}: McpDetailProps) {
  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>{key}</h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${key}`} />
      </div>
      {command && <div className={pageStyles.detailDesc}>{command}</div>}
      <div className={pageStyles.configSection}>
        <p className={styles.noSettings}>MCP 服务器配置请在 Advanced 页面管理。</p>
      </div>
    </div>
  );
}
