import { useMemo, useCallback, useState } from 'react';
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
  { name: 'read_file', title: '读取文件', description: '读取文件内容（包括文本、代码、PDF、图像等）', icon: '📄', category: 'read' },
  { name: 'read_multiple_files', title: '读取多个文件', description: '同时读取多个文件', icon: '📑', category: 'read' },
  { name: 'list_directory', title: '目录', description: '列出文件夹中的文件和目录', icon: '📂', category: 'read' },
  { name: 'search_files', title: '搜索文件', description: '按文件名或内容搜索文件', icon: '🔍', category: 'read' },
  { name: 'get_file_info', title: '获取文件信息', description: '获取有关文件的详细元数据', icon: 'ℹ️', category: 'read' },
  { name: 'write_file', title: '创建文本文件', description: '创建新的文本文件，或覆盖现有的文本文件', icon: '💾', category: 'write' },
  { name: 'edit_file', title: '编辑文件', description: '对文本文件进行基于行的编辑', icon: '✏️', category: 'write' },
  { name: 'create_directory', title: '创建目录', description: '创建新的目录', icon: '📂', category: 'write' },
  { name: 'copy_file', title: '复制文件', description: '复制文件和目录', icon: '📋', category: 'write' },
  { name: 'move_file', title: '移动/重命名文件', description: '移动或重命名文件和目录', icon: '↔️', category: 'write' },
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

function parseDirs(raw: string): string[] {
  const out: string[] = [];
  for (const line of raw.split('\n')) {
    const trimmed = line.trim();
    if (trimmed) out.push(trimmed);
  }
  return out;
}

function serializeDirs(dirs: string[]): string {
  return dirs.filter(d => d.trim() !== '').join('\n');
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
  const [activeTab, setActiveTab] = useState<'tools' | 'permissions'>('tools');
  const [selectedDirIndex, setSelectedDirIndex] = useState<number | null>(null);

  const allowedDirsRaw = asString(getToolSettingValue('filesystem', 'allowed_directories', config));
  const allowedDirs = useMemo(() => parseDirs(allowedDirsRaw), [allowedDirsRaw]);

  const subtoolValues = useMemo(() => {
    const v: Record<string, boolean> = {};
    for (const st of SUBTOOLS) {
      const val = getToolSettingValue('filesystem', st.name, config);
      v[st.name] = val === undefined ? true : asBool(val);
    }
    return v;
  }, [config]);

  const handleSubtoolToggle = useCallback((subtoolName: string, next: boolean) => {
    setToolSettingValue('filesystem', subtoolName, next, config, onSectionField);
  }, [config, onSectionField]);

  const setAllowedDirs = useCallback((dirs: string[]) => {
    setToolSettingValue('filesystem', 'allowed_directories', serializeDirs(dirs), config, onSectionField);
  }, [config, onSectionField]);

  const handleAddDir = useCallback(() => {
    const next = [...allowedDirs, ''];
    setAllowedDirs(next);
    setSelectedDirIndex(next.length - 1);
  }, [allowedDirs, setAllowedDirs]);

  const handleRemoveDir = useCallback(() => {
    if (allowedDirs.length === 0) return;
    const idx = selectedDirIndex !== null ? selectedDirIndex : allowedDirs.length - 1;
    if (idx < 0 || idx >= allowedDirs.length) {
      setSelectedDirIndex(null);
      return;
    }
    const next = allowedDirs.filter((_, i) => i !== idx);
    setAllowedDirs(next);
    setSelectedDirIndex(null);
  }, [allowedDirs, selectedDirIndex, setAllowedDirs]);

  const handleDirChange = useCallback((index: number, value: string) => {
    const next = allowedDirs.map((d, i) => (i === index ? value : d));
    setAllowedDirs(next);
  }, [allowedDirs, setAllowedDirs]);

  const handleDirBlur = useCallback((index: number, value: string) => {
    if (value.trim() === '') {
      const next = allowedDirs.filter((_, i) => i !== index);
      setAllowedDirs(next);
      setSelectedDirIndex(null);
    }
  }, [allowedDirs, setAllowedDirs]);

  const readTools = SUBTOOLS.filter(s => s.category === 'read');
  const writeTools = SUBTOOLS.filter(s => s.category === 'write');

  const contentDisabled = !enabled;

  return (
    <div className={pageStyles.detailContent}>
      <div className={pageStyles.detailHeader}>
        <h2 className={pageStyles.detailTitle}>
          <span className={pageStyles.detailEmoji}>📁</span>
          文件系统访问
          {toolset && (
            <span className={styles.toolsetLabel}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>

      <div
        className={contentDisabled ? styles.disabledOverlay : undefined}
        aria-hidden={contentDisabled || undefined}
        tabIndex={contentDisabled ? -1 : undefined}
      >
        <img
          src="/ui/filesystem-banner.png"
          alt="Filesystem access"
          className={styles.banner}
        />

        <div className={styles.warningBanner}>
          <div className={styles.warningIcon}>⚠️</div>
          <div>
            访问文件系统可能存在风险，因为它可能修改或删除文件。在启用之前，请务必查阅文档。
          </div>
        </div>

        {description && (
          <div className={styles.description}>{description}</div>
        )}

        <div className={styles.tabs} role="tablist">
          <button
            type="button"
            className={`${styles.tab} ${activeTab === 'tools' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('tools')}
            role="tab"
            aria-selected={activeTab === 'tools'}
          >
            🔧 可用工具
          </button>
          <button
            type="button"
            className={`${styles.tab} ${activeTab === 'permissions' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('permissions')}
            role="tab"
            aria-selected={activeTab === 'permissions'}
          >
            📁 权限配置
          </button>
        </div>

        <div className={styles.tabContent} role="tabpanel">
          {activeTab === 'tools' && (
            <>
              <div className={styles.subtoolGroup}>
                <div className={styles.sectionTitle}>📖 阅读操作</div>
                {readTools.map(st => (
                  <SubtoolCard
                    key={st.name}
                    def={st}
                    enabled={subtoolValues[st.name] !== false}
                    onToggle={(next) => handleSubtoolToggle(st.name, next)}
                    isWrite={false}
                    disabled={contentDisabled}
                  />
                ))}
              </div>

              <div className={styles.subtoolGroup}>
                <div className={styles.sectionTitle}>
                  <span className={styles.writeWarningIcon}>⚠️</span>
                  ✏️ 编写操作
                </div>
                {writeTools.map(st => (
                  <SubtoolCard
                    key={st.name}
                    def={st}
                    enabled={subtoolValues[st.name] !== false}
                    onToggle={(next) => handleSubtoolToggle(st.name, next)}
                    isWrite={true}
                    disabled={contentDisabled}
                  />
                ))}
              </div>
            </>
          )}

          {activeTab === 'permissions' && (
            <>
              <div className={styles.description}>
                定义文件系统代理可以访问哪些文件夹。代理只能在这些文件夹及其子目录中读取、写入和搜索。
              </div>
              <div className={styles.dirListContainer}>
                <div className={styles.dirListHeader}>
                  <div className={styles.dirListHeaderLeft}>
                    <span>📁</span>
                    <span>Name</span>
                  </div>
                  <div className={styles.dirListActions}>
                    <button
                      type="button"
                      className={styles.dirActionBtn}
                      onClick={handleAddDir}
                      aria-label="Add directory"
                      title="Add directory"
                      disabled={contentDisabled}
                    >
                      +
                    </button>
                    <button
                      type="button"
                      className={styles.dirActionBtn}
                      onClick={handleRemoveDir}
                      aria-label="Remove directory"
                      title="Remove directory"
                      disabled={contentDisabled || allowedDirs.length === 0}
                    >
                      −
                    </button>
                  </div>
                </div>
                {allowedDirs.map((dir, idx) => (
                  <div
                    key={idx}
                    className={`${styles.dirRow} ${selectedDirIndex === idx ? styles.dirRowSelected : ''}`}
                    onClick={() => setSelectedDirIndex(idx)}
                  >
                    <span className={styles.dirRowIcon}>📁</span>
                    <input
                      type="text"
                      className={styles.dirRowInput}
                      value={dir}
                      onChange={(e) => handleDirChange(idx, e.currentTarget.value)}
                      onBlur={(e) => handleDirBlur(idx, e.currentTarget.value)}
                      placeholder="/path/to/directory"
                      aria-label={`Directory ${idx + 1}`}
                      autoFocus={dir === ''}
                    />
                  </div>
                ))}
                {allowedDirs.length === 0 && (
                  <div className={`${styles.dirRow} ${styles.emptyRow}`}>
                    暂无配置目录，点击 + 添加
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function SubtoolCard({
  def,
  enabled,
  onToggle,
  isWrite,
  disabled,
}: {
  def: SubtoolDef;
  enabled: boolean;
  onToggle: (next: boolean) => void;
  isWrite: boolean;
  disabled?: boolean;
}) {
  return (
    <div
      className={`${styles.subtoolCard} ${enabled ? '' : styles.subtoolCardDisabled} ${isWrite ? styles.subtoolCardWrite : ''}`}
    >
      <div className={styles.subtoolInfo}>
        <span className={styles.subtoolIcon}>{def.icon}</span>
        <div>
          <div className={styles.subtoolTitle}>{def.title}</div>
          <div className={styles.subtoolDesc}>{def.description}</div>
        </div>
      </div>
      <Switch checked={enabled} onChange={onToggle} ariaLabel={def.title} disabled={disabled} />
    </div>
  );
}
