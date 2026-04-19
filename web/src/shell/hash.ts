import { GROUP_IDS, type GroupId } from './groups';

export interface ParsedHash {
  group: GroupId | null;
  sub: string | null;
}

function safeDecode(raw: string): string {
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}

export function parseHash(hash: string): ParsedHash {
  const raw = hash.replace(/^#/, '');
  if (!raw) return { group: null, sub: null };
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (!GROUP_IDS.has(groupPart as GroupId)) return { group: null, sub: null };
  const subPart = slash === -1 ? null : raw.substring(slash + 1);
  const sub = subPart ? safeDecode(subPart) : null;
  return { group: groupPart as GroupId, sub };
}

export function stringifyHash(group: GroupId | null, sub: string | null): string {
  if (!group) return '';
  if (!sub) return '#' + group;
  return '#' + group + '/' + encodeURIComponent(sub);
}

export function migrateLegacyHash(
  hash: string,
  knownPlatformKeys: readonly string[],
): string | null {
  const raw = hash.replace(/^#/, '');
  if (!raw) return null;
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (GROUP_IDS.has(groupPart as GroupId)) return null;
  const legacyKey = safeDecode(raw);
  if (knownPlatformKeys.includes(legacyKey)) {
    return stringifyHash('gateway', legacyKey);
  }
  return null;
}
