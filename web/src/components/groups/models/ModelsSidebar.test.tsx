import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import ModelsSidebar from './ModelsSidebar';

function baseProps(
  overrides: Partial<ComponentProps<typeof ModelsSidebar>> = {},
): ComponentProps<typeof ModelsSidebar> {
  return {
    instances: [
      { key: 'anthropic_main', type: 'anthropic' },
      { key: 'openai_bot', type: 'openai' },
    ],
    activeSubKey: null,
    dirtyKeys: new Set<string>(),
    onSelectScalar: vi.fn(),
    onSelectInstance: vi.fn(),
    onNewProvider: vi.fn(),
    fallbackProviders: [],
    dirtyFallbackIndices: new Set<number>(),
    activeFallbackIndex: null,
    onSelectFallback: vi.fn(),
    onAddFallback: vi.fn(),
    onMoveFallback: vi.fn(),
    ...overrides,
  };
}

describe('ModelsSidebar', () => {
  it('renders the Default Model scalar row plus one row per instance', () => {
    render(<ModelsSidebar {...baseProps()} />);
    expect(screen.getByText('Default model')).toBeInTheDocument();
    expect(screen.getByText('anthropic_main')).toBeInTheDocument();
    expect(screen.getByText('openai_bot')).toBeInTheDocument();
  });

  it('fires onSelectScalar("model") when Default model is clicked', async () => {
    const user = userEvent.setup();
    const onSelectScalar = vi.fn();
    render(<ModelsSidebar {...baseProps({ onSelectScalar })} />);
    await user.click(screen.getByText('Default model'));
    expect(onSelectScalar).toHaveBeenCalledWith('model');
  });

  it('fires onSelectInstance(key) when an instance row is clicked', async () => {
    const user = userEvent.setup();
    const onSelectInstance = vi.fn();
    render(<ModelsSidebar {...baseProps({ onSelectInstance })} />);
    await user.click(screen.getByText('anthropic_main'));
    expect(onSelectInstance).toHaveBeenCalledWith('anthropic_main');
  });

  it('fires onNewProvider() when the + button is clicked', async () => {
    const user = userEvent.setup();
    const onNewProvider = vi.fn();
    render(<ModelsSidebar {...baseProps({ onNewProvider })} />);
    await user.click(screen.getByRole('button', { name: /new provider/i }));
    expect(onNewProvider).toHaveBeenCalled();
  });

  it('shows a dirty dot on rows whose key is in dirtyKeys', () => {
    render(<ModelsSidebar {...baseProps({ dirtyKeys: new Set(['openai_bot']) })} />);
    const dirtyRow = screen.getByText('openai_bot').closest('button');
    expect(dirtyRow?.querySelector('[title="Unsaved changes"]')).toBeTruthy();
  });
});

describe('ModelsSidebar — fallback providers section', () => {
  it('renders the "Fallback Providers" header', () => {
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [] })} />);
    expect(screen.getByText('Fallback Providers')).toBeInTheDocument();
  });

  it('renders one row per fallback with #N position badge', () => {
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
        })}
      />,
    );
    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    const anthropics = screen.getAllByText('anthropic');
    expect(anthropics.length).toBeGreaterThan(0);
    const openais = screen.getAllByText('openai');
    expect(openais.length).toBeGreaterThan(0);
  });

  it('disables up/down at boundaries', () => {
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
        })}
      />,
    );
    const upButtons = screen.getAllByRole('button', { name: /move up/i });
    const downButtons = screen.getAllByRole('button', { name: /move down/i });
    expect(upButtons[0]).toBeDisabled();
    expect(downButtons[0]).toBeEnabled();
    expect(upButtons[1]).toBeEnabled();
    expect(downButtons[1]).toBeDisabled();
  });

  it('renders "+ Add fallback" button', () => {
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [] })} />);
    expect(screen.getByRole('button', { name: /add fallback/i })).toBeInTheDocument();
  });

  it('calls onAddFallback when the + button is clicked', async () => {
    const onAddFallback = vi.fn();
    render(<ModelsSidebar {...baseProps({ fallbackProviders: [], onAddFallback })} />);
    await userEvent.click(screen.getByRole('button', { name: /add fallback/i }));
    expect(onAddFallback).toHaveBeenCalled();
  });

  it('calls onSelectFallback with the index when a row body is clicked', async () => {
    const onSelectFallback = vi.fn();
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
          onSelectFallback,
        })}
      />,
    );
    // Click the openai row's body — there are two "openai" texts (provider rows
    // and fallback rows). Find the one inside the fallback body button.
    const matches = screen.getAllByText('openai');
    // Last match is the fallback row body (rendered after the Providers list).
    await userEvent.click(matches[matches.length - 1]);
    expect(onSelectFallback).toHaveBeenCalledWith(1);
  });

  it('calls onMoveFallback(i, "up" / "down") on the inline arrow buttons', async () => {
    const onMoveFallback = vi.fn();
    render(
      <ModelsSidebar
        {...baseProps({
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
          onMoveFallback,
        })}
      />,
    );
    await userEvent.click(screen.getAllByRole('button', { name: /move down/i })[0]);
    expect(onMoveFallback).toHaveBeenCalledWith(0, 'down');
    await userEvent.click(screen.getAllByRole('button', { name: /move up/i })[1]);
    expect(onMoveFallback).toHaveBeenCalledWith(1, 'up');
  });
});
