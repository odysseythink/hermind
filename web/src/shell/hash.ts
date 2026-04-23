import { GROUP_IDS, type GroupId } from './groups';

export type HashState =
  | { mode: 'chat' }
  | { mode: 'settings'; groupId: GroupId; sub?: string };

function safeDecode(raw: string): string {
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}

export function parseHash(hash: string): HashState {
  const raw = (hash || '').replace(/^#\/?/, '');
  if (raw === '' || raw === 'chat' || raw.startsWith('chat/')) {
    return { mode: 'chat' };
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

  // Legacy bare group hash: #models, #memory, etc. → canonicalize.
  const slash = raw.indexOf('/');
  const groupPart = slash === -1 ? raw : raw.substring(0, slash);
  if (GROUP_IDS.has(groupPart as GroupId)) {
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
  if (s.mode === 'chat') return '#/chat';
  if (s.sub) return `#/settings/${s.groupId}/${encodeURIComponent(s.sub)}`;
  return `#/settings/${s.groupId}`;
}
