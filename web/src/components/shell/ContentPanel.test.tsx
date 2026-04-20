import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ComponentProps } from 'react';
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
