import { GROUP_IDS, type GroupId } from './groups';

export type HashState =
  | { mode: 'chat'; sessionId?: string }
  | { mode: 'settings'; groupId: GroupId; sub?: string };

const LEGACY_GROUPS: readonly GroupId[] = [
  'models',
  'gateway',
  'memory',
  'skills',
  'runtime',
  'advanced',
  'observability',
];

function safeDecode(raw: string): string {
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}

export function parseHash(hash: string): HashState {
  const raw = (hash || '').replace(/^#\/?/, '');
  if (raw === '' || raw === 'chat') return { mode: 'chat' };

  if (raw.startsWith('chat/')) {
    const id = raw.slice('chat/'.length);
    return id ? { mode: 'chat', sessionId: safeDecode(id) } : { mode: 'chat' };
  }

  if (raw === 'settings') return { mode: 'settings', groupId: 'models' };

  if (raw.startsWith('settings/')) {
    const rest = raw.slice('settings/'.length);
    const slash = rest.indexOf('/');
    const groupPart = slash === -1 ? rest : rest.substring(0, slash);
    if (!GROUP_IDS.has(groupPart as GroupId)) {
      return { mode: 'settings', groupId: 'models' };
    }
    if (slash === -1) {
      return { mode: 'settings', groupId: groupPart as GroupId };
    }
    const subPart = rest.substring(slash + 1);
    return { mode: 'settings', groupId: groupPart as GroupId, sub: safeDecode(subPart) };
  }

  // Legacy bare group hash: #models, #gateway, #gateway/feishu-bot-main
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (LEGACY_GROUPS.includes(groupPart as GroupId)) {
    if (slash === -1) {
      return { mode: 'settings', groupId: groupPart as GroupId };
    }
    return {
      mode: 'settings',
      groupId: groupPart as GroupId,
      sub: safeDecode(raw.substring(slash + 1)),
    };
  }

  return { mode: 'chat' };
}

export function stringifyHash(s: HashState): string {
  if (s.mode === 'chat') {
    return s.sessionId ? `#/chat/${encodeURIComponent(s.sessionId)}` : '#/chat';
  }
  if (s.sub) {
    return `#/settings/${s.groupId}/${encodeURIComponent(s.sub)}`;
  }
  return `#/settings/${s.groupId}`;
}

// migrateLegacyHash inspects an incoming hash and returns a canonical
// two-mode URL for it, or null if no migration is needed. It also
// resolves bare platform keys (the old #feishu-bot-main shape that
// predates group prefixes) against the known platform key set.
export function migrateLegacyHash(
  hash: string,
  knownPlatformKeys: readonly string[],
): string | null {
  const raw = (hash || '').replace(/^#\/?/, '');
  if (!raw) return null;

  // New-form hashes never need migration.
  if (raw === 'chat' || raw.startsWith('chat/')) return null;
  if (raw === 'settings' || raw.startsWith('settings/')) return null;

  // Legacy group-prefixed hash: canonicalize to #/settings/<group>[/<sub>]
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (LEGACY_GROUPS.includes(groupPart as GroupId)) {
    const parsed = parseHash(hash);
    return stringifyHash(parsed);
  }

  // Legacy bare platform key (#feishu-bot-main with no #gateway/ prefix).
  const legacyKey = safeDecode(raw);
  if (knownPlatformKeys.includes(legacyKey)) {
    return stringifyHash({ mode: 'settings', groupId: 'gateway', sub: legacyKey });
  }
  return null;
}
