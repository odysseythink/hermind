import { useMemo, useCallback } from 'react';
import pageStyles from '../../SkillToolsConfigPage.module.css';
import styles from './FilesystemConfig.module.css';
import Switch from '../../../../fields/Switch';
import type { ToolDetailProps } from '../types';

interface SubtoolDef {
  name: string;
  title: string;
  description: string;
  icon: string;
  category: 'read' | 'write';
}

const SUBTOOLS: SubtoolDef[] = [
  { name: 'read_file', title: '读取文件', description: '读取单个文件的内容', icon: '📄', category: 'read' },
  { name: 'read_multiple_files', title: '批量读取文件', description: '同时读取多个文件', icon: '📑', category: 'read' },
  { name: 'list_directory', title: '列出目录', description: '列出目录中的文件和子目录', icon: '📂', category: 'read' },
  { name: 'search_files', title: '搜索文件', description: '按 glob 模式递归搜索文件', icon: '🔍', category: 'read' },
  { name: 'get_file_info', title: '获取文件信息', description: '获取文件的元数据（大小、权限等）', icon: 'ℹ️', category: 'read' },
  { name: 'write_file', title: '写入文件', description: '将内容写入文件', icon: '💾', category: 'write' },
  { name: 'edit_file', title: '编辑文件', description: '在文件中查找并替换文本', icon: '✏️', category: 'write' },
  { name: 'create_directory', title: '创建目录', description: '创建目录（支持递归）', icon: '📂', category: 'write' },
  { name: 'copy_file', title: '复制文件', description: '复制文件到目标路径', icon: '📋', category: 'write' },
  { name: 'move_file', title: '移动文件', description: '移动或重命名文件', icon: '↔️', category: 'write' },
];

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

function asBool(v: unknown): boolean {
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v === 'true';
  return false;
}

function getToolSettingValue(
  toolName: string,
  fieldName: string,
  config?: Record<string, unknown>,
): unknown {
  const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
    | Record<string, Record<string, unknown>>
    | undefined);
  return settings?.[toolName]?.[fieldName];
}

function setToolSettingValue(
  toolName: string,
  fieldName: string,
  value: unknown,
  config: Record<string, unknown> | undefined,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  if (!onSectionField) return;
  const toolsCfg = (config?.tools as Record<string, unknown> | undefined) ?? {};
  const settings = (toolsCfg.settings as Record<string, Record<string, unknown>> | undefined) ?? {};
  const nextToolSettings = { ...(settings[toolName] ?? {}), [fieldName]: value };
  const nextSettings = { ...settings, [toolName]: nextToolSettings };
  onSectionField('tools', 'settings', nextSettings);
}

export default function FilesystemConfig({
  name,
  description,
  toolset,
  enabled,
  onToggle,
  config,
  onSectionField,
}: ToolDetailProps) {
  const allowedDirs = asString(getToolSettingValue('filesystem', 'allowed_directories', config));

  const subtoolValues = useMemo(() => {
    const v: Record<string, boolean> = {};
    for (const st of SUBTOOLS) {
      v[st.name] = asBool(getToolSettingValue('filesystem', st.name, config));
      if (v[st.name] !== true && v[st.name] !== false) {
        v[st.name] = true;
      }
    }
    return v;
  }, [config]);

  const handleDirChange = useCallback((value: string) => {
    setToolSettingValue('filesystem', 'allowed_directories', value, config, onSectionField);
  }, [config, onSectionField]);

  const handleSubtoolToggle = useCallback((subtoolName: string, next: boolean) => {
    setToolSettingValue('filesystem', subtoolName, next, config, onSectionField);
  }, [config, onSectionField]);

  const readTools = SUBTOOLS.filter(s => s.category === 'read');
  const writeTools = SUBTOOLS.filter(s => s.category === 'write');

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          <span className={pageStyles.detailEmoji}>📁</span>
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

      <div className={styles.warningBanner}>
        <div className={styles.warningIcon}>⚠️</div>
        <div className={styles.warningText}>
          访问文件系统可能存在风险，因为它可能修改或删除文件。在启用之前，请务必查阅文档。
        </div>
      </div>

      <div className={pageStyles.configSection}>
        <h3 className={styles.sectionTitle}>配置</h3>

        <div className={styles.configRow}>
          <div>
            <div className={styles.label}>Allowed directories</div>
            <div className={styles.help}>每行一个绝对路径。只允许访问这些目录下的文件。</div>
          </div>
          <textarea
            className={styles.textarea}
            value={allowedDirs}
            onChange={(e) => handleDirChange(e.currentTarget.value)}
            rows={4}
            placeholder="/home/user/projects&#10;/tmp"
            aria-label="Allowed directories"
          />
        </div>
      </div>

      <div className={pageStyles.configSection}>
        <h3 className={styles.sectionTitle}>可用工具</h3>

        <div className={styles.subtoolGroup}>
          <p className={styles.subtoolGroupLabel}>📖 阅读操作</p>
          {readTools.map(st => (
            <SubtoolRow
              key={st.name}
              def={st}
              enabled={subtoolValues[st.name] !== false}
              onToggle={(next) => handleSubtoolToggle(st.name, next)}
              isWrite={false}
            />
          ))}
        </div>

        <div className={styles.subtoolGroup}>
          <p className={styles.subtoolGroupLabel}>
            <span className={styles.writeWarningIcon}>⚠️</span>
            ✏️ 写入操作
          </p>
          {writeTools.map(st => (
            <SubtoolRow
              key={st.name}
              def={st}
              enabled={subtoolValues[st.name] !== false}
              onToggle={(next) => handleSubtoolToggle(st.name, next)}
              isWrite={true}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function SubtoolRow({
  def,
  enabled,
  onToggle,
  isWrite,
}: {
  def: SubtoolDef;
  enabled: boolean;
  onToggle: (next: boolean) => void;
  isWrite: boolean;
}) {
  return (
    <div className={`${styles.subtoolRow} ${enabled ? '' : styles.subtoolDisabled} ${isWrite ? styles.subtoolWrite : ''}`}>
      <div className={styles.subtoolInfo}>
        <span className={styles.subtoolIcon}>{def.icon}</span>
        <div>
          <div className={styles.subtoolTitle}>{def.title}</div>
          <div className={styles.subtoolDesc}>{def.description}</div>
        </div>
      </div>
      <Switch checked={enabled} onChange={onToggle} ariaLabel={def.title} />
    </div>
  );
}
