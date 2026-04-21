import styles from './SettingsSidebar.module.css';
import { useTranslation } from 'react-i18next';
import { GROUPS, type GroupId } from '../../shell/groups';
import GroupSection from './GroupSection';
import GatewaySidebar from '../groups/gateway/GatewaySidebar';
import SectionList from './SectionList';
import ModelsSidebar from '../groups/models/ModelsSidebar';
import AdvancedSidebar from '../groups/advanced/AdvancedSidebar';
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
  providerInstances: Array<{ key: string; type: string }>;
  dirtyProviderKeys: Set<string>;
  fallbackProviders: Array<{ provider: string }>;
  dirtyFallbackIndices: Set<number>;
  onSelectGroup: (id: GroupId) => void;
  onSelectSub: (key: string) => void;
  onToggleGroup: (id: GroupId) => void;
  onNewInstance: () => void;
  onNewProvider: () => void;
  onAddFallback: () => void;
  onMoveFallback: (index: number, direction: 'up' | 'down') => void;
  onReorderFallback: (from: number, to: number) => void;
  mcpInstances: Array<{ key: string; command: string; enabled: boolean }>;
  dirtyMcpKeys: Set<string>;
  onAddMcpServer: () => void;
  cronJobs: Array<{ name: string; schedule: string }>;
  dirtyCronIndices: Set<number>;
  onAddCronJob: () => void;
  onMoveCron: (index: number, direction: 'up' | 'down') => void;
}

export default function SettingsSidebar(props: SidebarProps) {
  const { t } = useTranslation('ui');
  return (
    <aside className={styles.sidebar} aria-label={t('sidebar.ariaLabel')}>
      {GROUPS.map(g => {
        const body =
          g.id === 'gateway' ? (
            <GatewaySidebar
              instances={props.instances}
              selectedKey={props.selectedKey}
              descriptors={props.descriptors}
              dirtyKeys={props.dirtyInstanceKeys}
              onSelect={key => {
                props.onSelectGroup('gateway');
                props.onSelectSub(key);
              }}
              onNewInstance={props.onNewInstance}
            />
          ) : g.id === 'models' ? (
            <ModelsSidebar
              instances={props.providerInstances}
              activeSubKey={props.activeGroup === 'models' ? props.activeSubKey : null}
              dirtyKeys={props.dirtyProviderKeys}
              onSelectScalar={key => {
                props.onSelectGroup('models');
                props.onSelectSub(key);
              }}
              onSelectInstance={key => {
                props.onSelectGroup('models');
                props.onSelectSub(key);
              }}
              onNewProvider={props.onNewProvider}
              fallbackProviders={props.fallbackProviders}
              dirtyFallbackIndices={props.dirtyFallbackIndices}
              activeFallbackIndex={(() => {
                if (props.activeGroup !== 'models') return null;
                if (!props.activeSubKey || !props.activeSubKey.startsWith('fallback:')) return null;
                const n = Number(props.activeSubKey.slice('fallback:'.length));
                return Number.isInteger(n) ? n : null;
              })()}
              onSelectFallback={i => {
                props.onSelectGroup('models');
                props.onSelectSub(`fallback:${i}`);
              }}
              onAddFallback={props.onAddFallback}
              onMoveFallback={props.onMoveFallback}
              onReorderFallback={props.onReorderFallback}
            />
          ) : g.id === 'advanced' ? (
            <AdvancedSidebar
              activeSubKey={props.activeGroup === 'advanced' ? props.activeSubKey : null}
              onSelectScalar={key => {
                props.onSelectGroup('advanced');
                props.onSelectSub(key);
              }}
              mcpInstances={props.mcpInstances}
              dirtyMcpKeys={props.dirtyMcpKeys}
              onSelectMcp={key => {
                props.onSelectGroup('advanced');
                props.onSelectSub(`mcp:${key}`);
              }}
              onAddMcpServer={props.onAddMcpServer}
              cronJobs={props.cronJobs}
              dirtyCronIndices={props.dirtyCronIndices}
              activeCronIndex={(() => {
                if (props.activeGroup !== 'advanced') return null;
                if (!props.activeSubKey || !props.activeSubKey.startsWith('cron:')) return null;
                const n = Number(props.activeSubKey.slice('cron:'.length));
                return Number.isInteger(n) ? n : null;
              })()}
              onSelectCron={i => {
                props.onSelectGroup('advanced');
                props.onSelectSub(`cron:${i}`);
              }}
              onAddCronJob={props.onAddCronJob}
              onMoveCron={props.onMoveCron}
            />
          ) : (
            <SectionList
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
