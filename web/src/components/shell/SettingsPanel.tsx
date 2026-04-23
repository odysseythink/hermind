import type { Config, ConfigSection as ConfigSectionT } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import ConfigSection from '../ConfigSection';
import ProviderEditor from '../groups/models/ProviderEditor';
import FallbackProviderEditor from '../groups/models/FallbackProviderEditor';
import ListElementInlineEditor from './ListElementInlineEditor';
import KeyedInstanceInlineEditor from './KeyedInstanceInlineEditor';

export interface ContentPanelProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  config: Config;
  originalConfig: Config;
  configSections: ConfigSectionT[];
  onSelectGroup: (id: GroupId) => void;
  onConfigField: (sectionKey: string, field: string, value: unknown) => void;
  onConfigScalar: (sectionKey: string, value: unknown) => void;
  onConfigKeyedField: (sectionKey: string, instanceKey: string, field: string, value: unknown) => void;
  onConfigKeyedDelete: (sectionKey: string, instanceKey: string) => void;
  onFetchModels: (instanceKey: string) => Promise<{ models: string[] }>;
  onConfigListField: (sectionKey: string, index: number, field: string, value: unknown) => void;
  onConfigListDelete: (sectionKey: string, index: number) => void;
  onConfigListMove: (sectionKey: string, index: number, direction: 'up' | 'down') => void;
  onFetchFallbackModels: (index: number) => Promise<{ models: string[] }>;
}

function shallowEqualInstance(
  a: Record<string, unknown> | undefined,
  b: Record<string, unknown> | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  const ak = Object.keys(a);
  const bk = Object.keys(b);
  if (ak.length !== bk.length) return false;
  for (const k of ak) if (a[k] !== b[k]) return false;
  return true;
}

export default function SettingsPanel(props: ContentPanelProps) {
  if (props.activeGroup === null) {
    return <EmptyState onSelectGroup={props.onSelectGroup} />;
  }
  if (props.activeSubKey) {
    const section = props.configSections.find(
      s => s.key === props.activeSubKey && s.group_id === props.activeGroup,
    );
    if (section) {
      if (section.shape === 'scalar') {
        const scalar = (props.config as Record<string, unknown>)[section.key];
        const originalScalar = (props.originalConfig as Record<string, unknown>)[section.key];
        const field = section.fields[0];
        return (
          <ConfigSection
            section={section}
            value={{ [field.name]: scalar }}
            originalValue={{ [field.name]: originalScalar }}
            onFieldChange={(_name, v) => props.onConfigScalar(section.key, v)}
            config={props.config as unknown as Record<string, unknown>}
          />
        );
      }
      if (section.shape === 'keyed_map' || section.shape === 'list') {
        // activeSubKey is the section key; no instance/element selected yet.
        return <EmptyState onSelectGroup={props.onSelectGroup} />;
      }
      const value = (props.config as Record<string, unknown>)[section.key] as
        | Record<string, unknown>
        | undefined;
      const original = (props.originalConfig as Record<string, unknown>)[section.key] as
        | Record<string, unknown>
        | undefined;
      return (
        <ConfigSection
          section={section}
          value={value ?? {}}
          originalValue={original ?? {}}
          onFieldChange={(field, v) => props.onConfigField(section.key, field, v)}
          config={props.config as unknown as Record<string, unknown>}
        />
      );
    }
    // fallback:N addresses the N-th element of fallback_providers.
    if (props.activeGroup === 'models' && props.activeSubKey.startsWith('fallback:')) {
      const index = Number(props.activeSubKey.slice('fallback:'.length));
      const fbSection = props.configSections.find(s => s.key === 'fallback_providers');
      const list = ((props.config as Record<string, unknown>).fallback_providers as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      const origList = ((props.originalConfig as Record<string, unknown>).fallback_providers as
        | Array<Record<string, unknown>>
        | undefined) ?? [];
      if (
        fbSection &&
        fbSection.shape === 'list' &&
        Number.isInteger(index) &&
        index >= 0 &&
        index < list.length
      ) {
        const element = list[index];
        const originalElement = origList[index];
        const dirty = !shallowEqualInstance(element, originalElement);
        return (
          <FallbackProviderEditor
            sectionKey="fallback_providers"
            index={index}
            length={list.length}
            section={fbSection}
            value={element}
            originalValue={originalElement ?? {}}
            dirty={dirty}
            onField={(i, field, v) =>
              props.onConfigListField('fallback_providers', i, field, v)
            }
            onDelete={() => props.onConfigListDelete('fallback_providers', index)}
            onMoveUp={() => props.onConfigListMove('fallback_providers', index, 'up')}
            onMoveDown={() => props.onConfigListMove('fallback_providers', index, 'down')}
            fetchModels={() => props.onFetchFallbackModels(index)}
            config={props.config as unknown as Record<string, unknown>}
          />
        );
      }
    }
    // cron:N addresses the N-th element of config.cron.jobs (Subkey="jobs").
    if (props.activeGroup === 'advanced' && props.activeSubKey.startsWith('cron:')) {
      const index = Number(props.activeSubKey.slice('cron:'.length));
      const cronSection = props.configSections.find(s => s.key === 'cron');
      const list = (((props.config as Record<string, unknown>).cron as
        | { jobs?: Array<Record<string, unknown>> }
        | undefined)?.jobs) ?? [];
      const origList = (((props.originalConfig as Record<string, unknown>).cron as
        | { jobs?: Array<Record<string, unknown>> }
        | undefined)?.jobs) ?? [];
      if (
        cronSection && cronSection.shape === 'list' &&
        Number.isInteger(index) && index >= 0 && index < list.length
      ) {
        const element = list[index];
        const originalElement = origList[index];
        const dirty = !shallowEqualInstance(element, originalElement);
        return (
          <ListElementInlineEditor
            section={cronSection}
            index={index}
            length={list.length}
            value={element}
            originalValue={originalElement ?? {}}
            dirty={dirty}
            config={props.config as unknown as Record<string, unknown>}
            onField={(i, field, v) =>
              props.onConfigListField('cron', i, field, v)
            }
            onDelete={() => props.onConfigListDelete('cron', index)}
            onMoveUp={() => props.onConfigListMove('cron', index, 'up')}
            onMoveDown={() => props.onConfigListMove('cron', index, 'down')}
          />
        );
      }
    }
    // mcp:<name> addresses config.mcp.servers[<name>] (Subkey="servers").
    if (props.activeGroup === 'advanced' && props.activeSubKey.startsWith('mcp:')) {
      const instKey = props.activeSubKey.slice('mcp:'.length);
      const mcpSection = props.configSections.find(s => s.key === 'mcp');
      const servers = ((props.config as Record<string, unknown>).mcp as
        { servers?: Record<string, Record<string, unknown>> } | undefined)?.servers ?? {};
      const origServers = ((props.originalConfig as Record<string, unknown>).mcp as
        { servers?: Record<string, Record<string, unknown>> } | undefined)?.servers ?? {};
      const instance = servers[instKey];
      if (mcpSection && mcpSection.shape === 'keyed_map' && instance) {
        const dirty = !shallowEqualInstance(instance, origServers[instKey]);
        return (
          <KeyedInstanceInlineEditor
            section={mcpSection}
            instanceKey={instKey}
            value={instance}
            originalValue={origServers[instKey] ?? {}}
            dirty={dirty}
            config={props.config as unknown as Record<string, unknown>}
            onField={(key, field, v) => props.onConfigKeyedField('mcp', key, field, v)}
            onDelete={() => props.onConfigKeyedDelete('mcp', instKey)}
          />
        );
      }
    }
    // gateway:<name> addresses config.gateway.platforms[<name>] (Subkey="platforms").
    if (props.activeGroup === 'gateway' && props.activeSubKey.startsWith('gateway:')) {
      const instKey = props.activeSubKey.slice('gateway:'.length);
      const gatewaySection = props.configSections.find(s => s.key === 'gateway');
      const platforms = ((props.config as Record<string, unknown>).gateway as
        { platforms?: Record<string, Record<string, unknown>> } | undefined)?.platforms ?? {};
      const origPlatforms = ((props.originalConfig as Record<string, unknown>).gateway as
        { platforms?: Record<string, Record<string, unknown>> } | undefined)?.platforms ?? {};
      const instance = platforms[instKey];
      if (gatewaySection && gatewaySection.shape === 'keyed_map' && instance) {
        const dirty = !shallowEqualInstance(instance, origPlatforms[instKey]);
        return (
          <KeyedInstanceInlineEditor
            section={gatewaySection}
            instanceKey={instKey}
            value={instance}
            originalValue={origPlatforms[instKey] ?? {}}
            dirty={dirty}
            config={props.config as unknown as Record<string, unknown>}
            onField={(key, field, v) => props.onConfigKeyedField('gateway', key, field, v)}
            onDelete={() => props.onConfigKeyedDelete('gateway', instKey)}
          />
        );
      }
    }
    // Key didn't match a section — try treating it as a provider-instance key.
    const providersSection = props.configSections.find(s => s.key === 'providers');
    if (
      providersSection &&
      providersSection.shape === 'keyed_map' &&
      props.activeGroup === 'models'
    ) {
      const providers = ((props.config as Record<string, unknown>).providers ?? {}) as Record<
        string,
        Record<string, unknown>
      >;
      const origProviders = ((props.originalConfig as Record<string, unknown>).providers ?? {}) as Record<
        string,
        Record<string, unknown>
      >;
      const instance = providers[props.activeSubKey];
      if (instance) {
        const dirty = !shallowEqualInstance(instance, origProviders[props.activeSubKey]);
        return (
          <ProviderEditor
            sectionKey="providers"
            instanceKey={props.activeSubKey}
            section={providersSection}
            value={instance}
            originalValue={origProviders[props.activeSubKey] ?? {}}
            dirty={dirty}
            onField={(instKey, field, v) =>
              props.onConfigKeyedField('providers', instKey, field, v)
            }
            onDelete={() => props.onConfigKeyedDelete('providers', props.activeSubKey!)}
            fetchModels={() => props.onFetchModels(props.activeSubKey!)}
            config={props.config as unknown as Record<string, unknown>}
          />
        );
      }
    }
  }
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
