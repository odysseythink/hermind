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
