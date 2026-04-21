import { describe, it, expect } from 'vitest';
import { parseHash, stringifyHash, migrateLegacyHash } from './hash';

describe('parseHash', () => {
  it('empty hash → chat mode', () => {
    expect(parseHash('')).toEqual({ mode: 'chat' });
    expect(parseHash('#')).toEqual({ mode: 'chat' });
  });

  it('#/chat → chat mode', () => {
    expect(parseHash('#/chat')).toEqual({ mode: 'chat' });
  });

  it('#/chat/<id> → chat with sessionId', () => {
    expect(parseHash('#/chat/abc-123')).toEqual({ mode: 'chat', sessionId: 'abc-123' });
  });

  it('#/settings → settings default group', () => {
    expect(parseHash('#/settings')).toEqual({ mode: 'settings', groupId: 'models' });
  });

  it('#/settings/<group>', () => {
    expect(parseHash('#/settings/gateway')).toEqual({ mode: 'settings', groupId: 'gateway' });
  });

  it('#/settings/<group>/<sub> keeps sub', () => {
    expect(parseHash('#/settings/gateway/feishu-bot-main')).toEqual({
      mode: 'settings',
      groupId: 'gateway',
      sub: 'feishu-bot-main',
    });
  });

  it('decodes percent-encoded sub keys', () => {
    const enc = encodeURIComponent('key with/special chars');
    expect(parseHash('#/settings/gateway/' + enc)).toEqual({
      mode: 'settings',
      groupId: 'gateway',
      sub: 'key with/special chars',
    });
  });

  it('legacy bare group #models → settings/models', () => {
    expect(parseHash('#models')).toEqual({ mode: 'settings', groupId: 'models' });
  });

  it('legacy #gateway/<sub> keeps sub', () => {
    expect(parseHash('#gateway/feishu-bot-main')).toEqual({
      mode: 'settings',
      groupId: 'gateway',
      sub: 'feishu-bot-main',
    });
  });

  it('unknown hashes fall back to chat', () => {
    expect(parseHash('#bogus')).toEqual({ mode: 'chat' });
  });

  it('unknown settings group falls back to models', () => {
    expect(parseHash('#/settings/nope')).toEqual({ mode: 'settings', groupId: 'models' });
  });
});

describe('stringifyHash', () => {
  it('chat with no id', () => {
    expect(stringifyHash({ mode: 'chat' })).toBe('#/chat');
  });

  it('chat with id', () => {
    expect(stringifyHash({ mode: 'chat', sessionId: 'abc' })).toBe('#/chat/abc');
  });

  it('settings + group only', () => {
    expect(stringifyHash({ mode: 'settings', groupId: 'memory' })).toBe('#/settings/memory');
  });

  it('settings + group + sub (encoded)', () => {
    expect(stringifyHash({ mode: 'settings', groupId: 'gateway', sub: 'feishu-bot-main' }))
      .toBe('#/settings/gateway/feishu-bot-main');
    expect(stringifyHash({ mode: 'settings', groupId: 'gateway', sub: 'weird/key' }))
      .toBe('#/settings/gateway/' + encodeURIComponent('weird/key'));
  });

  it('round-trips through parseHash', () => {
    const s = stringifyHash({ mode: 'settings', groupId: 'gateway', sub: 'weird/key with%' });
    expect(parseHash(s)).toEqual({
      mode: 'settings',
      groupId: 'gateway',
      sub: 'weird/key with%',
    });
  });
});

describe('migrateLegacyHash', () => {
  const platforms = ['feishu-bot-main', 'dingtalk-alerts'];

  it('returns null for empty hash', () => {
    expect(migrateLegacyHash('', platforms)).toBeNull();
  });

  it('returns null for already-canonical chat hashes', () => {
    expect(migrateLegacyHash('#/chat', platforms)).toBeNull();
    expect(migrateLegacyHash('#/chat/abc', platforms)).toBeNull();
  });

  it('returns null for already-canonical settings hashes', () => {
    expect(migrateLegacyHash('#/settings', platforms)).toBeNull();
    expect(migrateLegacyHash('#/settings/gateway/foo', platforms)).toBeNull();
  });

  it('canonicalizes bare legacy group hashes', () => {
    expect(migrateLegacyHash('#models', platforms)).toBe('#/settings/models');
    expect(migrateLegacyHash('#gateway/feishu-bot-main', platforms))
      .toBe('#/settings/gateway/feishu-bot-main');
  });

  it('migrates bare legacy platform key in platforms', () => {
    expect(migrateLegacyHash('#feishu-bot-main', platforms))
      .toBe('#/settings/gateway/feishu-bot-main');
  });

  it('returns null for unknown bare keys', () => {
    expect(migrateLegacyHash('#never-existed', platforms)).toBeNull();
  });
});
