import styles from './Sidebar.module.css';
import { GROUPS, type GroupId } from '../../shell/groups';
import GroupSection from './GroupSection';
import GatewaySidebar from '../groups/gateway/GatewaySidebar';
import SectionList from './SectionList';
import type { ConfigSection, SchemaDescriptor } from '../../api/schemas';

export interface SidebarProps {
  activeGroup: GroupId | null;
  activeSubKey: string | null;
  expandedGroups: Set<GroupId>;
  dirtyGroups: Set<GroupId>;
  instances: Array<{ key: string; type: string; enabled: boolean }>;
  selectedKey: string | null;
  descriptors: SchemaDescriptor[];
  configSections: ConfigSection[];
  dirtyInstanceKeys: Set<string>;
  onSelectGroup: (id: GroupId) => void;
  onSelectSub: (key: string) => void;
  onToggleGroup: (id: GroupId) => void;
  onNewInstance: () => void;
}

export default function Sidebar(props: SidebarProps) {
  return (
    <aside className={styles.sidebar} aria-label="Configuration groups">
      {GROUPS.map(g => {
        const body =
          g.id === 'gateway' ? (
            <GatewaySidebar
              instances={props.instances}
              selectedKey={props.selectedKey}
              descriptors={props.descriptors}
              dirtyKeys={props.dirtyInstanceKeys}
              onSelect={props.onSelectSub}
              onNewInstance={props.onNewInstance}
            />
          ) : (
            <SectionList
              group={g.id}
              sections={props.configSections.filter(s => s.group_id === g.id)}
              activeSubKey={props.activeGroup === g.id ? props.activeSubKey : null}
              onSelect={key => {
                props.onSelectGroup(g.id);
                props.onSelectSub(key);
              }}
            />
          );
        return (
          <GroupSection
            key={g.id}
            group={g.id}
            expanded={props.expandedGroups.has(g.id)}
            active={props.activeGroup === g.id}
            dirty={props.dirtyGroups.has(g.id)}
            onToggle={() => props.onToggleGroup(g.id)}
            onSelectGroup={props.onSelectGroup}
          >
            {body}
          </GroupSection>
        );
      })}
    </aside>
  );
}
