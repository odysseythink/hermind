import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ComponentProps } from 'react';
import GatewayPanel from './GatewayPanel';

function makeProps(overrides: Partial<ComponentProps<typeof GatewayPanel>> = {}) {
  return {
    selectedKey: null,
    instance: null,
    originalInstance: null,
    descriptor: null,
    dirty: false,
    busy: false,
    onField: vi.fn(),
    onToggleEnabled: vi.fn(),
    onDelete: vi.fn(),
    onApply: vi.fn(),
    ...overrides,
  };
}

describe('GatewayPanel', () => {
  it('shows a breadcrumb with the selected key when one exists', () => {
    const { container } = render(
      <GatewayPanel
        {...makeProps({
          selectedKey: 'feishu-main',
          instance: { type: 'feishu', enabled: true, options: {} },
          descriptor: { type: 'feishu', display_name: 'Feishu', fields: [] },
        })}
      />,
    );
    const crumbs = container.querySelector('[class*="crumbs"]');
    expect(crumbs?.textContent).toContain('Gateway');
    expect(crumbs?.textContent).toContain('feishu-main');
  });

  it('shows a generic breadcrumb when no instance is selected', () => {
    render(<GatewayPanel {...makeProps()} />);
    expect(screen.getByText(/gateway/i)).toBeInTheDocument();
  });

  it('renders GatewayApplyButton', () => {
    render(<GatewayPanel {...makeProps()} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeInTheDocument();
  });

  it('passes dirty flag through to the apply button', () => {
    render(<GatewayPanel {...makeProps({ dirty: true })} />);
    expect(screen.getByRole('button', { name: /apply/i })).toBeDisabled();
  });
});
