import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ComponentProps } from 'react';
import ContentPanel from './ContentPanel';
import type { Config } from '../../api/schemas';

const emptyCfg: Config = {};

function makeProps(
  overrides: Partial<ComponentProps<typeof ContentPanel>> = {},
): ComponentProps<typeof ContentPanel> {
  return {
    activeGroup: null,
    config: emptyCfg,
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
