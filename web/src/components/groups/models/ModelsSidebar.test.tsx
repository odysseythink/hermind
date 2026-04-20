import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ModelsSidebar from './ModelsSidebar';

describe('ModelsSidebar', () => {
  const baseProps = {
    instances: [
      { key: 'anthropic_main', type: 'anthropic' },
      { key: 'openai_bot', type: 'openai' },
    ],
    activeSubKey: null as string | null,
    dirtyKeys: new Set<string>(),
    onSelectScalar: () => {},
    onSelectInstance: () => {},
    onNewProvider: () => {},
  };

  it('renders the Default Model scalar row plus one row per instance', () => {
    render(<ModelsSidebar {...baseProps} />);
    expect(screen.getByText('Default model')).toBeInTheDocument();
    expect(screen.getByText('anthropic_main')).toBeInTheDocument();
    expect(screen.getByText('openai_bot')).toBeInTheDocument();
  });

  it('fires onSelectScalar("model") when Default model is clicked', async () => {
    const user = userEvent.setup();
    const onSelectScalar = vi.fn();
    render(<ModelsSidebar {...baseProps} onSelectScalar={onSelectScalar} />);
    await user.click(screen.getByText('Default model'));
    expect(onSelectScalar).toHaveBeenCalledWith('model');
  });

  it('fires onSelectInstance(key) when an instance row is clicked', async () => {
    const user = userEvent.setup();
    const onSelectInstance = vi.fn();
    render(<ModelsSidebar {...baseProps} onSelectInstance={onSelectInstance} />);
    await user.click(screen.getByText('anthropic_main'));
    expect(onSelectInstance).toHaveBeenCalledWith('anthropic_main');
  });

  it('fires onNewProvider() when the + button is clicked', async () => {
    const user = userEvent.setup();
    const onNewProvider = vi.fn();
    render(<ModelsSidebar {...baseProps} onNewProvider={onNewProvider} />);
    await user.click(screen.getByRole('button', { name: /new provider/i }));
    expect(onNewProvider).toHaveBeenCalled();
  });

  it('shows a dirty dot on rows whose key is in dirtyKeys', () => {
    render(
      <ModelsSidebar
        {...baseProps}
        dirtyKeys={new Set(['openai_bot'])}
      />,
    );
    const dirtyRow = screen.getByText('openai_bot').closest('button');
    expect(dirtyRow?.querySelector('[title="Unsaved changes"]')).toBeTruthy();
  });
});
