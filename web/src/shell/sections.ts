import type { GroupId } from './groups';

export interface SectionDef {
  key: string;
  groupId: GroupId;
  /** Human-readable stage marker used by the Sidebar and ComingSoonPanel. */
  plannedStage: string;
}

export const SECTIONS: readonly SectionDef[] = [
  { key: 'storage', groupId: 'runtime', plannedStage: 'done' },
] as const;

export function sectionsInGroup(id: GroupId): readonly SectionDef[] {
  return SECTIONS.filter(s => s.groupId === id);
}

export function findSection(key: string): SectionDef | undefined {
  return SECTIONS.find(s => s.key === key);
}
