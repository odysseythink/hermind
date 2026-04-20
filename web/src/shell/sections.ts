import type { GroupId } from './groups';

export interface SectionDef {
  key: string;
  groupId: GroupId;
  /** Human-readable stage marker used by the Sidebar and ComingSoonPanel. */
  plannedStage: string;
}

// Declaration order drives the Sidebar's subsection list inside each group.
// Within a group, simpler sections come before complex ones so the visual
// weight ramps up top-to-bottom.
export const SECTIONS: readonly SectionDef[] = [
  // runtime
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
  { key: 'agent', groupId: 'runtime', plannedStage: 'done' },
  { key: 'terminal', groupId: 'runtime', plannedStage: 'done' },
  // observability
  { key: 'logging', groupId: 'observability', plannedStage: 'done' },
  { key: 'metrics', groupId: 'observability', plannedStage: 'done' },
  { key: 'tracing', groupId: 'observability', plannedStage: 'done' },
] as const;

export function sectionsInGroup(id: GroupId): readonly SectionDef[] {
  return SECTIONS.filter(s => s.groupId === id);
}

export function findSection(key: string): SectionDef | undefined {
  return SECTIONS.find(s => s.key === key);
}
