import { useEffect, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './SkillToolsConfigPage.module.css';
import Switch from '../../fields/Switch';
import { apiFetch, ApiError } from '../../../api/client';
import {
  SkillsResponseSchema,
  ToolsResponseSchema,
  type ConfigSection as ConfigSectionT,
  type ConfigField,
} from '../../../api/schemas';
import { useDescriptorT } from '../../../i18n/useDescriptorT';

export interface SkillToolsConfigPageProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onField: (field: string, value: unknown) => void;
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void;
  config?: Record<string, unknown>;
}

function asBool(v: unknown): boolean {
  if (typeof v === 'boolean') return v;
  if (typeof v === 'string') return v === 'true';
  return false;
}

function asString(v: unknown): string {
  if (v === undefined || v === null) return '';
  return typeof v === 'string' ? v : String(v);
}

function parseIntField(raw: string): number {
  if (raw === '') return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

interface SkillItem {
  name: string;
  description?: string;
  enabled: boolean;
  settings_schema?: ConfigField[];
}

interface ToolItem {
  name: string;
  description?: string;
  toolset?: string;
  enabled: boolean;
  settings_schema?: ConfigField[];
}

type FetchState<T> =
  | { status: 'loading' }
  | { status: 'ok'; data: T }
  | { status: 'error'; message: string };

type SelectedItem =
  | { type: 'skill'; name: string }
  | { type: 'tool'; name: string }
  | { type: 'mcp'; name: string }
  | null;

export default function SkillToolsConfigPage(props: SkillToolsConfigPageProps) {
  const { t } = useTranslation('ui');
  const dt = useDescriptorT();
  const [skillsState, setSkillsState] = useState<FetchState<SkillItem[]>>({ status: 'loading' });
  const [toolsState, setToolsState] = useState<FetchState<ToolItem[]>>({ status: 'loading' });
  const [selected, setSelected] = useState<SelectedItem>(null);

  const loadSkills = useCallback((signal?: AbortSignal) => {
    setSkillsState({ status: 'loading' });
    apiFetch('/api/skills', { schema: SkillsResponseSchema, signal })
      .then(resp => setSkillsState({ status: 'ok', data: resp.skills }))
      .catch(err => {
        if (signal?.aborted) return;
        const msg = err instanceof ApiError ? `${err.status}` : err instanceof Error ? err.message : String(err);
        setSkillsState({ status: 'error', message: msg });
      });
  }, []);

  const loadTools = useCallback((signal?: AbortSignal) => {
    setToolsState({ status: 'loading' });
    apiFetch('/api/tools', { schema: ToolsResponseSchema, signal })
      .then(resp => setToolsState({ status: 'ok', data: resp.tools }))
      .catch(err => {
        if (signal?.aborted) return;
        const msg = err instanceof ApiError ? `${err.status}` : err instanceof Error ? err.message : String(err);
        setToolsState({ status: 'error', message: msg });
      });
  }, []);

  useEffect(() => {
    const ac = new AbortController();
    loadSkills(ac.signal);
    loadTools(ac.signal);
    return () => ac.abort();
  }, [loadSkills, loadTools]);

  // ---- Global settings ----
  const handleGlobalBool = (field: string, next: boolean) => {
    props.onField(field, next);
  };
  const handleGlobalInt = (field: string, raw: string) => {
    props.onField(field, parseIntField(raw));
  };

  // ---- Skills disabled list ----
  const disabledSkills = (props.value.disabled as string[] | undefined) ?? [];
  const disabledSkillSet = new Set(disabledSkills);

  const toggleSkill = (name: string, nextEnabled: boolean) => {
    const next = nextEnabled
      ? disabledSkills.filter(n => n !== name)
      : [...disabledSkills.filter(n => n !== name), name].sort();
    props.onField('disabled', next);
  };

  // ---- Tools disabled list ----
  const toolsDisabled = (props.config?.tools as Record<string, unknown> | undefined)?.disabled as string[] | undefined;
  const toolsDisabledSet = new Set(toolsDisabled ?? []);

  const toggleTool = (name: string, nextEnabled: boolean) => {
    if (!props.onSectionField) return;
    const toolsCfg = (props.config?.tools as Record<string, unknown> | undefined) ?? {};
    const cur = (toolsCfg.disabled as string[] | undefined) ?? [];
    const next = nextEnabled
      ? cur.filter(n => n !== name)
      : [...cur.filter(n => n !== name), name].sort();
    props.onSectionField('tools', 'disabled', next);
  };

  // ---- MCP servers ----
  const mcpServers = (props.config?.mcp as Record<string, unknown> | undefined)?.servers as
    | Record<string, Record<string, unknown>>
    | undefined;
  const mcpList = mcpServers
    ? Object.entries(mcpServers)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, srv]) => ({
          key,
          command: typeof srv.command === 'string' ? srv.command : '',
          enabled: srv.enabled !== false,
        }))
    : [];

  const toggleMcp = (key: string, nextEnabled: boolean) => {
    if (!props.onSectionField || !mcpServers) return;
    const next = { ...mcpServers, [key]: { ...mcpServers[key], enabled: nextEnabled } };
    props.onSectionField('mcp', 'servers', next);
  };

  const selectItem = (item: SelectedItem) => {
    setSelected(item);
  };

  const sectionLabel = dt.sectionLabel(props.section.key, props.section.label);

  return (
    <section className={styles.page} aria-label={sectionLabel}>
      {/* Global settings banner */}
      <div className={styles.globalBanner}>
        <h3 className={styles.globalTitle}>{t('skills.globals')}</h3>
        <div className={styles.globalRows}>
          <div className={styles.globalRow}>
            <div>
              <div className={styles.globalRowLabel}>
                {dt.fieldLabel(props.section.key, 'auto_extract', 'Auto-extract')}
              </div>
              <div className={styles.globalRowHelp}>
                {dt.fieldHelp(props.section.key, 'auto_extract', '')}
              </div>
            </div>
            <Switch
              checked={asBool(props.value.auto_extract)}
              onChange={(next) => handleGlobalBool('auto_extract', next)}
              ariaLabel={dt.fieldLabel(props.section.key, 'auto_extract', 'Auto-extract')}
            />
          </div>
          <div className={styles.globalRow}>
            <div>
              <div className={styles.globalRowLabel}>
                {dt.fieldLabel(props.section.key, 'inject_count', 'Inject count')}
              </div>
              <div className={styles.globalRowHelp}>
                {dt.fieldHelp(props.section.key, 'inject_count', '')}
              </div>
            </div>
            <input
              type="number"
              className={styles.numberInput}
              value={asString(props.value.inject_count)}
              onChange={(e) => handleGlobalInt('inject_count', e.currentTarget.value)}
              aria-label={dt.fieldLabel(props.section.key, 'inject_count', 'Inject count')}
            />
          </div>
          <div className={styles.globalRow}>
            <div>
              <div className={styles.globalRowLabel}>
                {dt.fieldLabel(props.section.key, 'generation_half_life', 'Generation half-life')}
              </div>
              <div className={styles.globalRowHelp}>
                {dt.fieldHelp(props.section.key, 'generation_half_life', '')}
              </div>
            </div>
            <input
              type="number"
              className={styles.numberInput}
              value={asString(props.value.generation_half_life)}
              onChange={(e) => handleGlobalInt('generation_half_life', e.currentTarget.value)}
              aria-label={dt.fieldLabel(props.section.key, 'generation_half_life', 'Generation half-life')}
            />
          </div>
        </div>
      </div>

      {/* Left: unified list */}
      <div className={styles.listPanel}>
        <GroupHeader icon="🤖" label={t('skills.list')} groupId="skillsGroup">
          {skillsState.status === 'loading' && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--muted)', fontSize: 'var(--fs-sm)' }}>{t('skills.loading')}</div>
          )}
          {skillsState.status === 'error' && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--error)', fontSize: 'var(--fs-sm)' }}>
              {t('skills.error', { msg: skillsState.message })}
            </div>
          )}
          {skillsState.status === 'ok' && skillsState.data.length === 0 && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--muted)', fontSize: 'var(--fs-sm)' }}>{t('skills.empty')}</div>
          )}
          {skillsState.status === 'ok' && skillsState.data.map(sk => {
            const enabled = !disabledSkillSet.has(sk.name);
            const isActive = selected?.type === 'skill' && selected.name === sk.name;
            return (
              <div
                key={sk.name}
                className={`${styles.itemRow} ${isActive ? styles.active : ''}`}
                onClick={() => selectItem({ type: 'skill', name: sk.name })}
              >
                <div className={styles.itemInfo}>
                  <div className={styles.itemName}>{sk.name}</div>
                  {sk.description && <div className={styles.itemDesc}>{sk.description}</div>}
                </div>
                <span className={`${styles.itemStatus} ${enabled ? styles.on : ''}`}>{enabled ? 'On' : 'Off'}</span>
                <span className={styles.itemArrow}>›</span>
              </div>
            );
          })}
        </GroupHeader>

        <GroupHeader icon="🛠️" label={t('tools.list')} groupId="toolsGroup">
          {toolsState.status === 'loading' && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--muted)', fontSize: 'var(--fs-sm)' }}>{t('tools.loading')}</div>
          )}
          {toolsState.status === 'error' && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--error)', fontSize: 'var(--fs-sm)' }}>
              {t('tools.error', { msg: toolsState.message })}
            </div>
          )}
          {toolsState.status === 'ok' && toolsState.data.length === 0 && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--muted)', fontSize: 'var(--fs-sm)' }}>{t('tools.empty')}</div>
          )}
          {toolsState.status === 'ok' && toolsState.data.map(tl => {
            const enabled = !toolsDisabledSet.has(tl.name);
            const isActive = selected?.type === 'tool' && selected.name === tl.name;
            return (
              <div
                key={tl.name}
                className={`${styles.itemRow} ${isActive ? styles.active : ''}`}
                onClick={() => selectItem({ type: 'tool', name: tl.name })}
              >
                <div className={styles.itemInfo}>
                  <div className={styles.itemName}>{tl.name}</div>
                  {tl.description && <div className={styles.itemDesc}>{tl.description}</div>}
                </div>
                <span className={`${styles.itemStatus} ${enabled ? styles.on : ''}`}>{enabled ? 'On' : 'Off'}</span>
                <span className={styles.itemArrow}>›</span>
              </div>
            );
          })}
        </GroupHeader>

        <GroupHeader icon="🔌" label="MCP Servers" groupId="mcpGroup">
          {mcpList.length === 0 && (
            <div style={{ padding: 'var(--space-2) var(--space-4)', color: 'var(--muted)', fontSize: 'var(--fs-sm)' }}>
              {t('advanced.mcp.empty', { defaultValue: 'No MCP servers configured.' })}
            </div>
          )}
          {mcpList.map(mcp => {
            const isActive = selected?.type === 'mcp' && selected.name === mcp.key;
            return (
              <div
                key={mcp.key}
                className={`${styles.itemRow} ${isActive ? styles.active : ''}`}
                onClick={() => selectItem({ type: 'mcp', name: mcp.key })}
              >
                <div className={styles.itemInfo}>
                  <div className={styles.itemName}>{mcp.key}</div>
                  {mcp.command && <div className={styles.itemDesc}>{mcp.command}</div>}
                </div>
                <span className={`${styles.itemStatus} ${mcp.enabled ? styles.on : ''}`}>{mcp.enabled ? 'On' : 'Off'}</span>
                <span className={styles.itemArrow}>›</span>
              </div>
            );
          })}
        </GroupHeader>
      </div>

      {/* Right: detail panel */}
      <div className={styles.detailPanel}>
        {!selected && (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>🤖</div>
            <div className={styles.emptyText}>{t('skills.selectItem', { defaultValue: 'Select a Skill, Tool, or MCP Server' })}</div>
          </div>
        )}
        {selected?.type === 'skill' && renderSkillDetail(selected.name, skillsState, disabledSkillSet, toggleSkill, props.config, props.onSectionField)}
        {selected?.type === 'tool' && renderToolDetail(selected.name, toolsState, toolsDisabledSet, toggleTool, props.config, props.onSectionField)}
        {selected?.type === 'mcp' && renderMcpDetail(selected.name, mcpList, toggleMcp, props.config, props.onSectionField)}
      </div>
    </section>
  );
}

/* ===== Sub-components ===== */

function GroupHeader({ icon, label, groupId, children }: { icon: string; label: string; groupId: string; children: React.ReactNode }) {
  const [expanded, setExpanded] = useState(true);
  return (
    <>
      <div
        className={`${styles.groupHeader} ${expanded ? '' : styles.collapsed}`}
        onClick={() => setExpanded(v => !v)}
      >
        <span>{icon}</span>
        <span>{label}</span>
        <span className={styles.chevron}>▼</span>
      </div>
      {expanded && <div id={groupId}>{children}</div>}
    </>
  );
}

function renderSkillDetail(
  name: string,
  state: FetchState<SkillItem[]>,
  disabledSet: Set<string>,
  onToggle: (name: string, enabled: boolean) => void,
  _config?: Record<string, unknown>,
  _onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  if (state.status !== 'ok') return null;
  const sk = state.data.find(s => s.name === name);
  if (!sk) return null;
  const enabled = !disabledSet.has(sk.name);
  return (
    <div className={styles.detailContent}>
      <div className={styles.detailHeader}>
        <h2 className={styles.detailTitle}>{sk.name}</h2>
        <Switch checked={enabled} onChange={(next) => onToggle(sk.name, next)} ariaLabel={`Enable ${sk.name}`} />
      </div>
      {sk.description && <div className={styles.detailDesc}>{sk.description}</div>}
      <div className={styles.tabs}>
        <div className={`${styles.tab} ${styles.active}`}>配置</div>
        <div className={styles.tab}>权限</div>
      </div>
      <div className={styles.configSection}>
        <h3>个性化配置</h3>
        {sk.settings_schema && sk.settings_schema.length > 0
          ? renderSchemaFields(name, sk.settings_schema, _config, _onSectionField)
          : <p className={styles.noSettings}>此技能暂无配置项。</p>}
      </div>
    </div>
  );
}

function renderToolDetail(
  name: string,
  state: FetchState<ToolItem[]>,
  disabledSet: Set<string>,
  onToggle: (name: string, enabled: boolean) => void,
  config?: Record<string, unknown>,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  if (state.status !== 'ok') return null;
  const tl = state.data.find(t => t.name === name);
  if (!tl) return null;
  const enabled = !disabledSet.has(tl.name);
  return (
    <div className={styles.detailContent}>
      <div className={styles.detailHeader}>
        <h2 className={styles.detailTitle}>
          {tl.name}
          {tl.toolset && <span style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginLeft: 'var(--space-2)' }}>({tl.toolset})</span>}
        </h2>
        <Switch checked={enabled} onChange={(next) => onToggle(tl.name, next)} ariaLabel={`Enable ${tl.name}`} />
      </div>
      {tl.description && <div className={styles.detailDesc}>{tl.description}</div>}
      <div className={styles.tabs}>
        <div className={`${styles.tab} ${styles.active}`}>配置</div>
        <div className={styles.tab}>权限</div>
      </div>
      <div className={styles.configSection}>
        <h3>个性化配置</h3>
        {tl.settings_schema && tl.settings_schema.length > 0
          ? renderSchemaFields(name, tl.settings_schema, config, onSectionField)
          : <p className={styles.noSettings}>此工具暂无配置项。</p>}
      </div>
    </div>
  );
}

function renderMcpDetail(
  name: string,
  list: Array<{ key: string; command: string; enabled: boolean }>,
  onToggle: (key: string, enabled: boolean) => void,
  _config?: Record<string, unknown>,
  _onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  const mcp = list.find(m => m.key === name);
  if (!mcp) return null;
  return (
    <div className={styles.detailContent}>
      <div className={styles.detailHeader}>
        <h2 className={styles.detailTitle}>{mcp.key}</h2>
        <Switch checked={mcp.enabled} onChange={(next) => onToggle(mcp.key, next)} ariaLabel={`Enable ${mcp.key}`} />
      </div>
      {mcp.command && <div className={styles.detailDesc}>{mcp.command}</div>}
      <div className={styles.tabs}>
        <div className={`${styles.tab} ${styles.active}`}>配置</div>
        <div className={styles.tab}>权限</div>
      </div>
      <div className={styles.configSection}>
        <h3>个性化配置</h3>
        <p className={styles.noSettings}>MCP 服务器配置请在 Advanced 页面管理。</p>
      </div>
    </div>
  );
}

/* ===== Schema field renderer ===== */

function getSettingValue(toolName: string, fieldName: string, config?: Record<string, unknown>): unknown {
  // Special case: browser_control reads from browser_extension config
  if (toolName === 'browser_control') {
    const be = config?.browser_extension as Record<string, unknown> | undefined;
    if (be && fieldName in be) return be[fieldName];
  }
  // Generic: read from tools.settings.<tool_name>.<field>
  const settings = ((config?.tools as Record<string, unknown> | undefined)?.settings as
    | Record<string, Record<string, unknown>>
    | undefined);
  return settings?.[toolName]?.[fieldName];
}

function setSettingValue(toolName: string, fieldName: string, value: unknown, onSectionField?: (sectionKey: string, field: string, value: unknown) => void) {
  if (!onSectionField) return;
  if (toolName === 'browser_control') {
    onSectionField('browser_extension', fieldName, value);
    return;
  }
  // Generic: would need to read current tools.settings, merge, and write back
  // For now unsupported
}

function renderSchemaFields(
  toolName: string,
  schema: ConfigField[],
  config?: Record<string, unknown>,
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void,
) {
  return schema.map(field => {
    const value = getSettingValue(toolName, field.name, config);
    const label = field.label || field.name;
    const help = field.help || '';

    let input: React.ReactNode;
    if (field.kind === 'bool') {
      input = (
        <Switch
          checked={asBool(value)}
          onChange={(next) => setSettingValue(toolName, field.name, next, onSectionField)}
          ariaLabel={label}
        />
      );
    } else if (field.kind === 'int') {
      input = (
        <input
          type="number"
          className={styles.numberInput}
          value={asString(value)}
          onChange={(e) => setSettingValue(toolName, field.name, parseIntField(e.currentTarget.value), onSectionField)}
          aria-label={label}
        />
      );
    } else if (field.kind === 'secret') {
      input = (
        <input
          type="password"
          className={styles.numberInput}
          style={{ width: '280px', fontFamily: 'var(--font-mono)' }}
          value={asString(value)}
          onChange={(e) => setSettingValue(toolName, field.name, e.currentTarget.value, onSectionField)}
          placeholder={field.default as string}
          aria-label={label}
        />
      );
    } else {
      input = (
        <input
          type="text"
          className={styles.numberInput}
          style={{ width: '280px', fontFamily: 'var(--font-mono)' }}
          value={asString(value)}
          onChange={(e) => setSettingValue(toolName, field.name, e.currentTarget.value, onSectionField)}
          placeholder={field.default as string}
          aria-label={label}
        />
      );
    }

    return (
      <div key={field.name} className={styles.configRow}>
        <div>
          <div className={styles.label}>{label}</div>
          {help && <div className={styles.help}>{help}</div>}
        </div>
        {input}
      </div>
    );
  });
}
