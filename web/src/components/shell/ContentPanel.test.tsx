import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState, type ComponentProps } from 'react';
import ContentPanel from './ContentPanel';
import type { Config, ConfigSection } from '../../api/schemas';

const emptyCfg: Config = {};

function makeProps(
  overrides: Partial<ComponentProps<typeof ContentPanel>> = {},
): ComponentProps<typeof ContentPanel> {
  return {
    activeGroup: null,
    activeSubKey: null,
    config: emptyCfg,
    originalConfig: emptyCfg,
    configSections: [],
    // gateway-specific — defaults acceptable when activeGroup !== 'gateway'
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirtyGateway: false,
    busy: false,
    onField: () => {},
    onToggleEnabled: () => {},
    onDelete: () => {},
    onApply: () => {},
    onSelectGroup: () => {},
    onConfigField: () => {},
    onConfigScalar: () => {},
    onConfigKeyedField: () => {},
    onConfigKeyedDelete: () => {},
    onFetchModels: async () => ({ models: [] }),
    onConfigListField: () => {},
    onConfigListDelete: () => {},
    onConfigListMove: () => {},
    onFetchFallbackModels: async () => ({ models: [] }),
    ...overrides,
  };
}

describe('ContentPanel', () => {
  it('renders EmptyState when activeGroup is null', () => {
    render(<ContentPanel {...makeProps()} />);
    expect(screen.getByText(/select a configuration section/i)).toBeInTheDocument();
  });

  it('renders GatewayPanel when activeGroup is gateway', () => {
    render(<ContentPanel {...makeProps({ activeGroup: 'gateway' })} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
  });

  it('renders ComingSoonPanel for every non-gateway group', () => {
    for (const id of ['models', 'memory', 'skills', 'runtime', 'advanced', 'observability'] as const) {
      const { unmount } = render(<ContentPanel {...makeProps({ activeGroup: id })} />);
      expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
      unmount();
    }
  });
});

describe('ContentPanel — non-gateway section routing', () => {
  const storageSection: ConfigSection = {
    key: 'storage',
    label: 'Storage',
    summary: 'Where hermind keeps data.',
    group_id: 'runtime',
    fields: [
      { name: 'driver', label: 'Driver', kind: 'enum',
        required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
      { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
        visible_when: { field: 'driver', equals: 'sqlite' } },
    ],
  };

  const baseProps = {
    activeGroup: 'runtime' as const,
    activeSubKey: 'storage',
    config: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
    originalConfig: { storage: { driver: 'sqlite', sqlite_path: '/var/db/x' } },
    configSections: [storageSection],
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirtyGateway: false,
    busy: false,
    onField: () => {},
    onToggleEnabled: () => {},
    onDelete: () => {},
    onApply: () => {},
    onSelectGroup: () => {},
    onConfigField: () => {},
    onConfigScalar: () => {},
    onConfigKeyedField: () => {},
    onConfigKeyedDelete: () => {},
    onFetchModels: async () => ({ models: [] }),
    onConfigListField: () => {},
    onConfigListDelete: () => {},
    onConfigListMove: () => {},
    onFetchFallbackModels: async () => ({ models: [] }),
  };

  it('renders the ConfigSection for runtime/storage', () => {
    render(<ContentPanel {...baseProps} />);
    expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey does not match a registered section', () => {
    render(
      <ContentPanel {...baseProps} activeSubKey="somethingelse" />,
    );
    // ComingSoonPanel shows "<GroupLabel> — coming soon"
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey is null', () => {
    render(
      <ContentPanel {...baseProps} activeSubKey={null} />,
    );
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });
});

describe('ContentPanel — scalar section routing', () => {
  const modelSection: ConfigSection = {
    key: 'model',
    label: 'Default model',
    group_id: 'models',
    shape: 'scalar',
    fields: [{ name: 'model', label: 'Model', kind: 'string', required: true }],
  };

  it('renders the field wrapped around the raw scalar value', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'model',
          config: { model: 'anthropic/claude-opus-4-6' } as unknown as Config,
          originalConfig: { model: 'anthropic/claude-opus-4-6' } as unknown as Config,
          configSections: [modelSection],
        })}
      />,
    );
    const input = screen.getByLabelText(/Model/) as HTMLInputElement;
    expect(input.value).toBe('anthropic/claude-opus-4-6');
  });

  it('dispatches onConfigScalar (not onConfigField) when the field changes', async () => {
    const user = userEvent.setup();
    const onConfigScalar = vi.fn();
    const onConfigField = vi.fn();
    // Stateful Host: controlled inputs need prop updates to reflect typing;
    // thread the scalar value back through `config` so userEvent.type observes
    // an up-to-date DOM value between keystrokes.
    function Host() {
      const [model, setModel] = useState('anthropic/claude-opus-4-6');
      return (
        <ContentPanel
          {...makeProps({
            activeGroup: 'models',
            activeSubKey: 'model',
            config: { model } as unknown as Config,
            originalConfig: { model: 'anthropic/claude-opus-4-6' } as unknown as Config,
            configSections: [modelSection],
            onConfigScalar: (sectionKey, v) => {
              setModel(v as string);
              onConfigScalar(sectionKey, v);
            },
            onConfigField,
          })}
        />
      );
    }
    render(<Host />);
    const input = screen.getByLabelText(/Model/);
    await user.clear(input);
    await user.type(input, 'anthropic/claude-opus-4-7');
    expect(onConfigField).not.toHaveBeenCalled();
    const calls = onConfigScalar.mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    const [sectionKey, value] = calls[calls.length - 1];
    expect(sectionKey).toBe('model');
    expect(value).toBe('anthropic/claude-opus-4-7');
  });
});

describe('ContentPanel — keyed_map section routing', () => {
  const providersSection: ConfigSection = {
    key: 'providers',
    label: 'Providers',
    group_id: 'models',
    shape: 'keyed_map',
    fields: [
      { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
        enum: ['anthropic'] },
      { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    ],
  };

  it('renders EmptyState when no activeSubKey is selected', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: null,
          configSections: [providersSection],
        })}
      />,
    );
    // EmptyState renders the "No instance selected" message (or similar).
    expect(screen.getByText(/select/i)).toBeInTheDocument();
  });

  it('renders ProviderEditor for a selected keyed-map instance', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'anthropic_main',
          config: {
            providers: {
              anthropic_main: { provider: 'anthropic', api_key: '' },
            },
          } as unknown as Config,
          originalConfig: {
            providers: {
              anthropic_main: { provider: 'anthropic', api_key: '' },
            },
          } as unknown as Config,
          configSections: [providersSection],
        })}
      />,
    );
    expect(screen.getByText(/anthropic_main/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
  });
});

describe('ContentPanel — list section routing', () => {
  const fbSection: ConfigSection = {
    key: 'fallback_providers',
    label: 'Fallback Providers',
    group_id: 'models',
    shape: 'list',
    fields: [
      { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
        enum: ['anthropic', 'openai'] },
      { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    ],
  };

  it('renders FallbackProviderEditor for a fallback:N subkey', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'fallback:1',
          config: {
            fallback_providers: [
              { provider: 'anthropic', api_key: '' },
              { provider: 'openai', api_key: '' },
            ],
          } as unknown as Config,
          originalConfig: {
            fallback_providers: [
              { provider: 'anthropic', api_key: '' },
              { provider: 'openai', api_key: '' },
            ],
          } as unknown as Config,
          configSections: [fbSection],
        })}
      />,
    );
    expect(screen.getByText(/fallback #2/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
  });

  it('does not render the editor when fallback:N index is out of bounds', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'fallback:5',
          config: { fallback_providers: [] } as unknown as Config,
          originalConfig: { fallback_providers: [] } as unknown as Config,
          configSections: [fbSection],
        })}
      />,
    );
    expect(screen.queryByText(/fallback #/i)).not.toBeInTheDocument();
  });

  it('passes onFetchFallbackModels callback through to the editor with the index', async () => {
    const fetchModels = vi.fn(async () => ({ models: ['m1'] }));
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'models',
          activeSubKey: 'fallback:0',
          config: {
            fallback_providers: [{ provider: 'anthropic', api_key: '' }],
          } as unknown as Config,
          originalConfig: {
            fallback_providers: [{ provider: 'anthropic', api_key: '' }],
          } as unknown as Config,
          configSections: [fbSection],
          onFetchFallbackModels: fetchModels,
        })}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /fetch models/i }));
    expect(fetchModels).toHaveBeenCalledWith(0);
  });
});

describe('ContentPanel — cron section routing', () => {
  const cronSection: ConfigSection = {
    key: 'cron',
    label: 'Cron jobs',
    group_id: 'advanced',
    shape: 'list',
    subkey: 'jobs',
    fields: [
      { name: 'name', label: 'Name', kind: 'string', required: true },
      { name: 'schedule', label: 'Schedule', kind: 'string', required: true },
      { name: 'prompt', label: 'Prompt', kind: 'string', required: true },
      { name: 'model', label: 'Model override', kind: 'string' },
    ],
  };

  const cronConfig = {
    cron: {
      jobs: [
        { name: 'daily', schedule: '0 9 * * *', prompt: 'hello', model: '' },
        { name: 'hourly', schedule: 'every 1h', prompt: 'check', model: '' },
      ],
    },
  } as unknown as Config;

  it('renders ListElementInlineEditor for cron:N subkey', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'advanced',
          activeSubKey: 'cron:1',
          config: cronConfig,
          originalConfig: cronConfig,
          configSections: [cronSection],
        })}
      />,
    );
    expect(screen.getByText(/#2 of 2/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
  });

  it('does not render the editor when cron:N index is out of bounds', () => {
    render(
      <ContentPanel
        {...makeProps({
          activeGroup: 'advanced',
          activeSubKey: 'cron:5',
          config: { cron: { jobs: [] } } as unknown as Config,
          originalConfig: { cron: { jobs: [] } } as unknown as Config,
          configSections: [cronSection],
        })}
      />,
    );
    expect(screen.queryByText(/#\d+ of/i)).not.toBeInTheDocument();
  });

  it('calls onConfigListField with sectionKey "cron" when a field changes', async () => {
    const onConfigListField = vi.fn();
    function Host() {
      const [jobs, setJobs] = useState([
        { name: 'daily', schedule: '0 9 * * *', prompt: 'hello', model: '' },
      ]);
      return (
        <ContentPanel
          {...makeProps({
            activeGroup: 'advanced',
            activeSubKey: 'cron:0',
            config: { cron: { jobs } } as unknown as Config,
            originalConfig: { cron: { jobs } } as unknown as Config,
            configSections: [cronSection],
            onConfigListField: (sk, idx, field, v) => {
              setJobs(prev => {
                const next = [...prev];
                next[idx] = { ...next[idx], [field]: v as string };
                return next;
              });
              onConfigListField(sk, idx, field, v);
            },
          })}
        />
      );
    }
    render(<Host />);
    const input = screen.getByLabelText(/name/i) as HTMLInputElement;
    await userEvent.clear(input);
    await userEvent.type(input, 'x');
    expect(onConfigListField).toHaveBeenCalledWith('cron', 0, 'name', expect.any(String));
  });
});
