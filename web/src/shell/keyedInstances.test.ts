import { describe, it, expect } from 'vitest';
import { keyedInstanceDirty } from './keyedInstances';
import { initialState, type AppState } from '../state';

function stateWith(current: any, original: any): AppState {
  return { ...initialState, config: current, originalConfig: original } as AppState;
}

describe('keyedInstanceDirty', () => {
  it('returns false when instance is identical between current and original', () => {
    const inst = { provider: 'anthropic', base_url: '', api_key: '', model: '' };
    const s = stateWith(
      { providers: { a: inst } },
      { providers: { a: inst } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(false);
  });

  it('returns true when a field diverges', () => {
    const s = stateWith(
      { providers: { a: { provider: 'anthropic', base_url: 'https://edited' } } },
      { providers: { a: { provider: 'anthropic', base_url: '' } } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns true for newly created instances (absent in original)', () => {
    const s = stateWith(
      { providers: { a: { provider: 'anthropic' } } },
      { providers: {} },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns true for deleted instances (absent in current)', () => {
    const s = stateWith(
      { providers: {} },
      { providers: { a: { provider: 'anthropic' } } },
    );
    expect(keyedInstanceDirty(s, 'providers', 'a')).toBe(true);
  });

  it('returns false when both current and original lack the instance', () => {
    const s = stateWith({ providers: {} }, { providers: {} });
    expect(keyedInstanceDirty(s, 'providers', 'absent')).toBe(false);
  });
});
