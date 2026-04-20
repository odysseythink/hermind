import { describe, it, expect } from 'vitest';
import { listInstanceDirty } from './listInstances';
import { initialState } from '../state';
import type { Config } from '../api/schemas';

function makeState(cfg: Config, orig: Config) {
  return { ...initialState, status: 'ready' as const, config: cfg, originalConfig: orig };
}

describe('listInstanceDirty', () => {
  it('returns false when element is identical to original', () => {
    const s = makeState(
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(false);
  });

  it('returns true when a field differs', () => {
    const s = makeState(
      { fallback_providers: [{ provider: 'anthropic', api_key: 'b' }] } as unknown as Config,
      { fallback_providers: [{ provider: 'anthropic', api_key: 'a' }] } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(true);
  });

  it('returns true for appended elements (no original counterpart)', () => {
    const s = makeState(
      {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
      {
        fallback_providers: [{ provider: 'anthropic', api_key: 'a' }],
      } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 1)).toBe(true);
  });

  it('returns true after a reorder (index-based comparison)', () => {
    const s = makeState(
      {
        fallback_providers: [
          { provider: 'openai', api_key: 'b' },
          { provider: 'anthropic', api_key: 'a' },
        ],
      } as unknown as Config,
      {
        fallback_providers: [
          { provider: 'anthropic', api_key: 'a' },
          { provider: 'openai', api_key: 'b' },
        ],
      } as unknown as Config,
    );
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(true);
    expect(listInstanceDirty(s, 'fallback_providers', 1)).toBe(true);
  });

  it('returns false when section is absent in both', () => {
    const s = makeState({} as Config, {} as Config);
    expect(listInstanceDirty(s, 'fallback_providers', 0)).toBe(false);
  });
});
