import type { Config, ConfigSection as ConfigSectionT, PlatformInstance, SchemaDescriptor } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import GatewayPanel from '../groups/gateway/GatewayPanel';
import ConfigSection from '../ConfigSection';

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
  }
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
