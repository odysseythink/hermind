import type { Config, PlatformInstance, SchemaDescriptor } from '../../api/schemas';
import { type GroupId } from '../../shell/groups';
import ComingSoonPanel from './ComingSoonPanel';
import EmptyState from './EmptyState';
import GatewayPanel from '../groups/gateway/GatewayPanel';

export interface ContentPanelProps {
  activeGroup: GroupId | null;
  config: Config;
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
  return <ComingSoonPanel group={props.activeGroup} config={props.config} />;
}
