import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState, type ComponentProps } from 'react';
import SettingsPanel from './SettingsPanel';
import type { Config, ConfigSection } from '../../api/schemas';

const emptyCfg: Config = {};

function makeProps(
  overrides: Partial<ComponentProps<typeof SettingsPanel>> = {},
): ComponentProps<typeof SettingsPanel> {
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

describe('SettingsPanel', () => {
  it('renders EmptyState when activeGroup is null', () => {
    render(<SettingsPanel {...makeProps()} />);
    expect(screen.getByText(/select a configuration section/i)).toBeInTheDocument();
  });

  it('renders GatewayPanel when activeGroup is gateway', () => {
    render(<SettingsPanel {...makeProps({ activeGroup: 'gateway' })} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
  });

  it('renders ComingSoonPanel for every non-gateway group', () => {
    for (const id of ['models', 'memory', 'skills', 'runtime', 'advanced', 'observability'] as const) {
      const { unmount } = render(<SettingsPanel {...makeProps({ activeGroup: id })} />);
      expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
      unmount();
    }
  });
});

describe('SettingsPanel — non-gateway section routing', () => {
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
    render(<SettingsPanel {...baseProps} />);
    expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey does not match a registered section', () => {
    render(
      <SettingsPanel {...baseProps} activeSubKey="somethingelse" />,
    );
    // ComingSoonPanel shows "<GroupLabel> — coming soon"
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });

  it('falls back to ComingSoonPanel when subKey is null', () => {
    render(
      <SettingsPanel {...baseProps} activeSubKey={null} />,
    );
    expect(screen.getByText(/runtime — coming soon/i)).toBeInTheDocument();
  });
});

describe('SettingsPanel — scalar section routing', () => {
  const modelSection: ConfigSection = {
    key: 'model',
    label: 'Default model',
    group_id: 'models',
    shape: 'scalar',
    fields: [{ name: 'model', label: 'Model', kind: 'string', required: true }],
  };

  it('renders the field wrapped around the raw scalar value', () => {
    render(
      <SettingsPanel
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
        <SettingsPanel
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

describe('SettingsPanel — keyed_map section routing', () => {
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
      <SettingsPanel
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
      <SettingsPanel
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

describe('SettingsPanel — list section routing', () => {
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
      <SettingsPanel
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
      <SettingsPanel
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
      <SettingsPanel
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

describe('SettingsPanel — cron section routing', () => {
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
      <SettingsPanel
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
      <SettingsPanel
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
        <SettingsPanel
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

describe('SettingsPanel — mcp section routing', () => {
  const mcpSection: ConfigSection = {
    key: 'mcp',
    label: 'MCP servers',
    group_id: 'advanced',
    shape: 'keyed_map',
    subkey: 'servers',
    no_discriminator: true,
    fields: [
      { name: 'command', label: 'Command', kind: 'string', required: true },
      { name: 'enabled', label: 'Enabled', kind: 'bool' },
    ],
  };

  const mcpConfig = {
    mcp: {
      servers: {
        filesystem: { command: 'npx', enabled: true },
        github: { command: '/usr/local/bin/mcp-github', enabled: false },
      },
    },
  } as unknown as Config;

  it('renders KeyedInstanceInlineEditor for mcp:<name> subkey', () => {
    render(
      <SettingsPanel
        {...makeProps({
          activeGroup: 'advanced',
          activeSubKey: 'mcp:filesystem',
          config: mcpConfig,
          originalConfig: mcpConfig,
          configSections: [mcpSection],
        })}
      />,
    );
    expect(screen.getByText('filesystem')).toBeInTheDocument();
    expect(screen.getByLabelText(/command/i)).toBeInTheDocument();
  });

  it('falls through to ComingSoonPanel when mcp instance key does not exist', () => {
    render(
      <SettingsPanel
        {...makeProps({
          activeGroup: 'advanced',
          activeSubKey: 'mcp:nonexistent',
          config: mcpConfig,
          originalConfig: mcpConfig,
          configSections: [mcpSection],
        })}
      />,
    );
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });

  it('calls onConfigKeyedField with sectionKey "mcp" when a field changes', async () => {
    const onConfigKeyedField = vi.fn();
    function Host() {
      const [servers, setServers] = useState<Record<string, Record<string, unknown>>>({
        filesystem: { command: 'npx', enabled: true },
      });
      return (
        <SettingsPanel
          {...makeProps({
            activeGroup: 'advanced',
            activeSubKey: 'mcp:filesystem',
            config: { mcp: { servers } } as unknown as Config,
            originalConfig: { mcp: { servers } } as unknown as Config,
            configSections: [mcpSection],
            onConfigKeyedField: (sk, instKey, field, v) => {
              setServers(prev => ({
                ...prev,
                [instKey]: { ...prev[instKey], [field]: v },
              }));
              onConfigKeyedField(sk, instKey, field, v);
            },
          })}
        />
      );
    }
    render(<Host />);
    const input = screen.getByLabelText(/command/i) as HTMLInputElement;
    await userEvent.clear(input);
    await userEvent.type(input, 'node');
    expect(onConfigKeyedField).toHaveBeenCalledWith('mcp', 'filesystem', 'command', expect.any(String));
  });
});
