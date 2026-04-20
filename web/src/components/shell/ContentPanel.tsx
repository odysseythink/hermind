import type { Config, ConfigSection as ConfigSectionT, PlatformInstance, SchemaDescriptor } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import GatewayPanel from '../groups/gateway/GatewayPanel';
import ConfigSection from '../ConfigSection';
import ProviderEditor from '../groups/models/ProviderEditor';

export interface ContentPanelProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  config: Config;
  originalConfig: Config;
  configSections: ConfigSectionT[];
  selectedKey: string | null;
  instance: PlatformInstance | null;
  originalInstance: PlatformInstance | null;
  descriptor: SchemaDescriptor | null;
  dirtyGateway: boolean;
  busy: boolean;
  onField: (field: string, value: string) => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
  onApply: () => void;
  onSelectGroup: (id: GroupId) => void;
  onConfigField: (sectionKey: string, field: string, value: unknown) => void;
  onConfigScalar: (sectionKey: string, value: unknown) => void;
  onConfigKeyedField: (sectionKey: string, instanceKey: string, field: string, value: unknown) => void;
  onConfigKeyedDelete: (sectionKey: string, instanceKey: string) => void;
  onFetchModels: (instanceKey: string) => Promise<{ models: string[] }>;
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

export default function ContentPanel(props: ContentPanelProps) {
  if (props.activeGroup === null) {
    return <EmptyState onSelectGroup={props.onSelectGroup} />;
  }
  if (props.activeGroup === 'gateway') {
    return (
      <GatewayPanel
        selectedKey={props.selectedKey}
        instance={props.instance}
        originalInstance={props.originalInstance}
        descriptor={props.descriptor}
        dirty={props.dirtyGateway}
        busy={props.busy}
        onField={props.onField}
        onToggleEnabled={props.onToggleEnabled}
        onDelete={props.onDelete}
        onApply={props.onApply}
      />
    );
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
          />
        );
      }
      if (section.shape === 'keyed_map') {
        // activeSubKey is the section key; no instance selected yet.
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
        />
      );
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
          />
        );
      }
    }
  }
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
