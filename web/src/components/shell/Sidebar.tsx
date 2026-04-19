import styles from './Sidebar.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';
import GroupSection from './GroupSection';
import GatewaySidebar from '../groups/gateway/GatewaySidebar';
import type { SchemaDescriptor } from '../../api/schemas';

export interface SidebarProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
  dirtyGroups: Set<GroupId>;
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  dirtyInstanceKeys: Set<string>;
  onSelectGroup: (id: GroupId) => void;
  onSelectSub: (key: string) => void;
  onToggleGroup: (id: GroupId) => void;
  onNewInstance: () => void;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <aside className={styles.sidebar} aria-label="Configuration groups">
      {GROUPS.map(g => (
        <GroupSection
          key={g.id}
          group={g.id}
          expanded={props.expandedGroups.has(g.id)}
          active={props.activeGroup === g.id}
          dirty={props.dirtyGroups.has(g.id)}
          onToggle={() => props.onToggleGroup(g.id)}
          onSelectGroup={props.onSelectGroup}
        >
          {g.id === 'gateway' && (
            <GatewaySidebar
              instances={props.instances}
              selectedKey={props.selectedKey}
              descriptors={props.descriptors}
              dirtyKeys={props.dirtyInstanceKeys}
              onSelect={props.onSelectSub}
              onNewInstance={props.onNewInstance}
            />
          )}
        </GroupSection>
      ))}
    </aside>
  );
}
