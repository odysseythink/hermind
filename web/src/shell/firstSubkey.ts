import type { ConfigSection } from '../api/schemas';
import type { GroupId } from './groups';

// Resolve a sensible default subkey for a group so a bare group
// selection (click on group header, or a hash like "#/settings/models"
// with no sub segment) lands on a real editor instead of the
// ComingSoonPanel fallback.
//
// Rules:
//   - models prefers the first configured provider instance so users land
//     on an editable credential form when providers already exist, and
//     falls back to the scalar "model" section otherwise.
//   - everyone else picks the first configSection whose shape is scalar or
//     map. keyed_map/list shapes require an element selection, so skipped.
export function firstSubkeyForGroup(
  group: GroupId,
  configSections: readonly ConfigSection[],
  providerKeys: readonly string[],
): string | null {
  if (group === 'models' && providerKeys.length > 0) {
    return providerKeys[0];
  }

  const candidates = configSections.filter((s) => s.group_id === group);
  const standalone = candidates.find(
    (s) => s.shape === 'scalar' || s.shape === 'map' || s.shape === undefined,
  );
  return standalone?.key ?? null;
}
