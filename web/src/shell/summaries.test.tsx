import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import type { Config } from '../api/schemas';
import { summaryFor } from './summaries';

const empty: Config = {};

describe('summaryFor', () => {
  it('returns null for the gateway group', () => {
    expect(summaryFor('gateway', empty)).toBeNull();
  });

  it('renders a models summary with default model + provider counts', () => {
    const cfg = {
      model: 'claude-opus-4-7',
      providers: { openai: {}, anthropic: {} },
      fallback_providers: [{ type: 'openai' }],
    } as unknown as Config;
    const { container } = render(<>{summaryFor('models', cfg)}</>);
    expect(container.textContent).toContain('claude-opus-4-7');
    expect(container.textContent).toContain('2'); // providers count
    expect(container.textContent).toContain('1'); // fallback count
  });

  it('renders a memory summary with backend + enabled flag', () => {
    const cfg = {
      memory: { enabled: true, retain_db: { endpoint: 'x' } },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('memory', cfg)}</>);
    expect(container.textContent).toContain('retain_db');
    expect(container.textContent).toMatch(/yes/i);
  });

  it('renders a skills summary with disabled + override counts', () => {
    const cfg = {
      skills: {
        disabled: ['a', 'b', 'c'],
        platform_disabled: { cli: ['x'] },
      },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('skills', cfg)}</>);
    expect(container.textContent).toContain('3');
    expect(container.textContent).toContain('1');
  });

  it('renders a runtime summary with storage kind + agent prompt presence', () => {
    const cfg = {
      agent: { prompt: 'custom prompt' },
      storage: { kind: 'sqlite' },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('runtime', cfg)}</>);
    expect(container.textContent).toContain('sqlite');
    expect(container.textContent).toContain('custom');
  });

  it('renders an advanced summary with MCP + cron counts', () => {
    const cfg = {
      mcp: { servers: { a: {}, b: {} } },
      cron: { jobs: [{ name: 'x' }, { name: 'y' }, { name: 'z' }] },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('advanced', cfg)}</>);
    expect(container.textContent).toContain('2');
    expect(container.textContent).toContain('3');
  });

  it('renders an observability summary with log level + enabled flags', () => {
    const cfg = {
      logging: { level: 'debug' },
      metrics: { enabled: true },
      tracing: { enabled: false },
    } as unknown as Config;
    const { container } = render(<>{summaryFor('observability', cfg)}</>);
    expect(container.textContent).toContain('debug');
    expect(container.textContent).toMatch(/on/i);
    expect(container.textContent).toMatch(/off/i);
  });

  it('renders gracefully on an empty config for every placeholder group', () => {
    for (const id of ['models', 'memory', 'skills', 'runtime', 'advanced', 'observability'] as const) {
      const { container } = render(<>{summaryFor(id, empty)}</>);
      expect(container.textContent).toBeTruthy();
    }
  });
});
