import { describe, it, expect } from 'vitest';
import { parseHash, stringifyHash, migrateLegacyHash } from './hash';

describe('parseHash', () => {
  it('returns null/null for an empty hash', () => {
    expect(parseHash('')).toEqual({ group: null, sub: null });
    expect(parseHash('#')).toEqual({ group: null, sub: null });
  });

  it('parses #<group> with no sub', () => {
    expect(parseHash('#models')).toEqual({ group: 'models', sub: null });
    expect(parseHash('#gateway')).toEqual({ group: 'gateway', sub: null });
  });

  it('parses #<group>/<sub>', () => {
    expect(parseHash('#gateway/feishu-bot-main')).toEqual({
      group: 'gateway',
      sub: 'feishu-bot-main',
    });
  });

  it('decodes percent-encoded sub keys', () => {
    expect(parseHash('#gateway/' + encodeURIComponent('key with/special chars'))).toEqual({
      group: 'gateway',
      sub: 'key with/special chars',
    });
  });

  it('returns null/null for unknown group names', () => {
    expect(parseHash('#bogus')).toEqual({ group: null, sub: null });
    expect(parseHash('#bogus/whatever')).toEqual({ group: null, sub: null });
  });

  it('tolerates malformed percent encoding by passing through', () => {
    // '%' alone is invalid percent-encoding; decodeURIComponent throws.
    expect(parseHash('#gateway/%-raw')).toEqual({ group: 'gateway', sub: '%-raw' });
  });
});

describe('stringifyHash', () => {
  it('returns empty for null group', () => {
    expect(stringifyHash(null, null)).toBe('');
    expect(stringifyHash(null, 'ignored')).toBe('');
  });

  it('builds #<group> when sub is null', () => {
    expect(stringifyHash('models', null)).toBe('#models');
  });

  it('builds #<group>/<sub> and encodes the sub', () => {
    expect(stringifyHash('gateway', 'feishu-bot-main')).toBe('#gateway/feishu-bot-main');
    expect(stringifyHash('gateway', 'key with/special')).toBe(
      '#gateway/' + encodeURIComponent('key with/special'),
    );
  });

  it('round-trips with parseHash', () => {
    const h = stringifyHash('gateway', 'weird/key with%');
    expect(parseHash(h)).toEqual({ group: 'gateway', sub: 'weird/key with%' });
  });
});

describe('migrateLegacyHash', () => {
  const platforms = ['feishu-bot-main', 'dingtalk-alerts'];

  it('returns null when hash is empty', () => {
    expect(migrateLegacyHash('', platforms)).toBeNull();
  });

  it('returns null when hash already matches a known group', () => {
    expect(migrateLegacyHash('#gateway/anything', platforms)).toBeNull();
    expect(migrateLegacyHash('#models', platforms)).toBeNull();
  });

  it('migrates a bare legacy key that exists in platforms', () => {
    expect(migrateLegacyHash('#feishu-bot-main', platforms)).toBe('#gateway/feishu-bot-main');
  });

  it('migrates percent-encoded legacy keys', () => {
    const legacy = '#' + encodeURIComponent('feishu-bot-main');
    expect(migrateLegacyHash(legacy, platforms)).toBe('#gateway/feishu-bot-main');
  });

  it('returns null for unknown bare keys', () => {
    expect(migrateLegacyHash('#never-existed', platforms)).toBeNull();
  });
});
