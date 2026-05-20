# Filesystem Config AnythingLLM-Style Port — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite `FilesystemConfig` to match AnythingLLM's filesystem skill panel with banner, warning, tabs (Available Tools / Permissions), card-style sub-tool toggles, and a managed directory list.

**Architecture:** Single-component refactor inside the existing detail-renderer registry. The component gains local UI state for tab switching and directory selection, while persisting all settings through the existing `config.tools.settings.filesystem` path. CSS Module is rewritten to provide the new visual styles.

**Tech Stack:** React 18 + TypeScript, CSS Modules, Vite, hermind design tokens (CSS custom properties).

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx` | Rewrite | Main component with header, banner, tabs, sub-tool cards, directory list |
| `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css` | Rewrite | All component-local styles |
| `web/public/filesystem-banner.png` | Already added | Static banner image ( AnythingLLM asset) |

---

## Task 1: Rewrite Stylesheet

**Files:**
- Rewrite: `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css`

- [ ] **Step 1: Write the new stylesheet**

Replace the entire file with:

```css
/* ===== Banner ===== */
.banner {
  width: 100%;
  max-height: 180px;
  object-fit: cover;
  border-radius: var(--r-lg);
  margin-bottom: var(--space-4);
}

/* ===== Warning Banner ===== */
.warningBanner {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: rgba(210, 153, 34, 0.08);
  border: 1px solid rgba(210, 153, 34, 0.25);
  border-radius: var(--r-md);
  margin-bottom: var(--space-4);
  font-size: var(--fs-sm);
  color: var(--warning);
}

.warningIcon {
  font-size: 1.25rem;
  flex-shrink: 0;
  margin-top: 0.125rem;
}

/* ===== Description ===== */
.description {
  font-size: var(--fs-sm);
  color: var(--muted);
  margin-bottom: var(--space-2);
  line-height: 1.5;
}

/* ===== Tab Content Wrapper ===== */
.tabContent {
  margin-top: var(--space-4);
}

/* ===== Section Title ===== */
.sectionTitle {
  font-size: var(--fs-xs);
  font-weight: 600;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: var(--space-2);
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.writeWarningIcon {
  color: #ea580c;
}

/* ===== Subtool Card ===== */
.subtoolGroup {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin-bottom: var(--space-5);
}

.subtoolCard {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3);
  border-radius: var(--r-lg);
  border: 1px solid var(--border);
  background: var(--surface-2);
  transition: opacity var(--t-fast) ease;
}

.subtoolCardDisabled {
  opacity: 0.45;
}

.subtoolCardWrite {
  background: rgba(234, 88, 12, 0.06);
  border-color: rgba(234, 88, 12, 0.25);
}

.subtoolInfo {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.subtoolIcon {
  font-size: 1.25rem;
  flex-shrink: 0;
}

.subtoolTitle {
  font-size: var(--fs-sm);
  font-weight: 500;
  color: var(--text);
}

.subtoolDesc {
  font-size: var(--fs-xs);
  color: var(--muted);
  margin-top: 1px;
}

/* ===== Permissions: Directory List ===== */
.dirListContainer {
  border: 1px solid var(--border);
  border-radius: var(--r-md);
  background: var(--surface);
  overflow: hidden;
}

.dirListHeader {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--border);
  font-size: var(--fs-xs);
  color: var(--muted);
  font-weight: 500;
}

.dirListHeaderLeft {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.dirListActions {
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.dirActionBtn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  background: var(--surface-2);
  color: var(--text);
  font-size: var(--fs-sm);
  cursor: pointer;
  transition: background var(--t-fast), border-color var(--t-fast);
  padding: 0;
  line-height: 1;
}

.dirActionBtn:hover {
  background: var(--hover-tint);
  border-color: var(--muted);
}

.dirActionBtn:active {
  background: var(--active-tint);
}

.dirRow {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background var(--t-fast);
}

.dirRow:last-child {
  border-bottom: none;
}

.dirRow:hover {
  background: var(--hover-tint);
}

.dirRowSelected {
  background: var(--accent-bg);
}

.dirRowIcon {
  font-size: var(--fs-sm);
  flex-shrink: 0;
  opacity: 0.7;
}

.dirRowInput {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  padding: 0;
  outline: none;
}

.dirRowInput::placeholder {
  color: var(--muted);
  opacity: 0.5;
}

/* ===== Disabled overlay ===== */
.disabledOverlay {
  opacity: 0.5;
  pointer-events: none;
  user-select: none;
}
```

- [ ] **Step 2: Commit stylesheet**

```bash
git add web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.module.css
git commit -m "style(filesystem): rewrite stylesheet for AnythingLLM-style panel"
```

---

## Task 2: Rewrite FilesystemConfig Component

**Files:**
- Rewrite: `web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx`

- [ ] **Step 1: Write the full component**

Replace the entire file with:

```tsx
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
      v[st.name] = asBool(getToolSettingValue('filesystem', st.name, config));
      if (v[st.name] !== true && v[st.name] !== false) {
        v[st.name] = true;
      }
    }
    return v;
  }, [config]);

  const handleSubtoolToggle = useCallback((subtoolName: string, next: boolean) => {
    setToolSettingValue('filesystem', subtoolName, next, config, onSectionField);
  }, [config, onSectionField]);

  // ---- Directory list mutations ----
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
            <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>
              ({toolset})
            </span>
          )}
        </h2>
        <Switch checked={enabled} onChange={onToggle} ariaLabel={`Enable ${name}`} />
      </div>

      <div className={contentDisabled ? styles.disabledOverlay : undefined}>
        <img
          src="/filesystem-banner.png"
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

        {/* Tabs */}
        <div className={pageStyles.tabs}>
          <button
            type="button"
            className={`${pageStyles.tab} ${activeTab === 'tools' ? pageStyles.active : ''}`}
            onClick={() => setActiveTab('tools')}
          >
            🔧 可用工具
          </button>
          <button
            type="button"
            className={`${pageStyles.tab} ${activeTab === 'permissions' ? pageStyles.active : ''}`}
            onClick={() => setActiveTab('permissions')}
          >
            📁 权限配置
          </button>
        </div>

        <div className={styles.tabContent}>
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
                    >
                      +
                    </button>
                    <button
                      type="button"
                      className={styles.dirActionBtn}
                      onClick={handleRemoveDir}
                      aria-label="Remove directory"
                      title="Remove directory"
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
                  <div
                    className={styles.dirRow}
                    style={{ color: 'var(--muted)', fontSize: 'var(--fs-sm)', cursor: 'default' }}
                  >
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
}: {
  def: SubtoolDef;
  enabled: boolean;
  onToggle: (next: boolean) => void;
  isWrite: boolean;
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
      <Switch checked={enabled} onChange={onToggle} ariaLabel={def.title} />
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: No errors in `FilesystemConfig.tsx`.

- [ ] **Step 3: Commit component**

```bash
git add web/src/components/groups/skills/detail-renderers/filesystem/FilesystemConfig.tsx
git commit -m "feat(filesystem): rewrite config panel with AnythingLLM-style tabs and directory list"
```

---

## Task 3: Build and Verify

**Files:** None (build verification only)

- [ ] **Step 1: Run production build**

```bash
cd web && npm run build
```

Expected: Build succeeds with no errors.

- [ ] **Step 2: Verify banner asset is included**

Check that `web/dist/filesystem-banner.png` (or `web/dist/assets/...`) exists after build.

```bash
ls web/dist/filesystem-banner.png 2>/dev/null || find web/dist -name "filesystem-banner*" | head -5
```

Expected: At least one match.

- [ ] **Step 3: Commit any build artifacts**

```bash
git add web/dist/
git commit -m "chore: rebuild web assets with new filesystem panel"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ Banner image — Task 2, JSX includes `<img src="/filesystem-banner.png">`
- ✅ Warning banner — Task 2, `styles.warningBanner`
- ✅ Tab navigation (Available Tools / Permissions) — Task 2, local state `activeTab`
- ✅ Sub-tool cards with read/write grouping — Task 2, `SubtoolCard` with category-based styles
- ✅ Write-action orange tint — Task 1, `.subtoolCardWrite`
- ✅ Directory list with [+] / [−] — Task 2, `handleAddDir` / `handleRemoveDir`
- ✅ Directory row selection — Task 2, `selectedDirIndex` + click handler
- ✅ Empty row auto-cleanup on blur — Task 2, `handleDirBlur`
- ✅ Master switch disables content — Task 2, `disabledOverlay` class
- ✅ Light/dark theme compatibility — Task 1, all colors use CSS custom properties or rgba overlays

**2. Placeholder scan:** No TBD, TODO, or vague steps found.

**3. Type consistency:** All helper signatures match usage. `setToolSettingValue` and `getToolSettingValue` reuse the existing pattern from the old component. `parseDirs`/`serializeDirs` are new but used consistently.

No gaps found — plan is complete.
