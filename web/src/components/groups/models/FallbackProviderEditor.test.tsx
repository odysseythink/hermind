import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import FallbackProviderEditor from './FallbackProviderEditor';
import type { ConfigSection } from '../../../api/schemas';

const section: ConfigSection = {
  key: 'fallback_providers',
  label: 'Fallback Providers',
  group_id: 'models',
  shape: 'list',
  fields: [
    { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
      enum: ['anthropic', 'openai'] },
    { name: 'base_url', label: 'Base URL', kind: 'string' },
    { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    { name: 'model', label: 'Model', kind: 'string' },
  ],
};

function baseProps(
  overrides: Partial<ComponentProps<typeof FallbackProviderEditor>> = {},
): ComponentProps<typeof FallbackProviderEditor> {
  return {
    sectionKey: 'fallback_providers',
    index: 0,
    length: 1,
    section,
    value: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
    originalValue: { provider: 'anthropic', base_url: '', api_key: '', model: '' },
    dirty: false,
    onField: vi.fn(),
    onDelete: vi.fn(),
    onMoveUp: vi.fn(),
    onMoveDown: vi.fn(),
    fetchModels: vi.fn(async () => ({ models: [] as string[] })),
    ...overrides,
  };
}

describe('FallbackProviderEditor', () => {
  it('renders a position header "Fallback #1" for index 0', () => {
    render(<FallbackProviderEditor {...baseProps()} />);
    expect(screen.getByText(/fallback #1/i)).toBeInTheDocument();
  });

  it('renders all 4 fields', () => {
    render(<FallbackProviderEditor {...baseProps()} />);
    expect(screen.getByLabelText(/provider type/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/base url/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^model/i)).toBeInTheDocument();
  });

  it('disables Up at index 0', () => {
    render(<FallbackProviderEditor {...baseProps({ index: 0, length: 3 })} />);
    expect(screen.getByRole('button', { name: /move up/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /move down/i })).toBeEnabled();
  });

  it('disables Down at the last index', () => {
    render(<FallbackProviderEditor {...baseProps({ index: 2, length: 3 })} />);
    expect(screen.getByRole('button', { name: /move up/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /move down/i })).toBeDisabled();
  });

  it('calls onDelete after confirm', async () => {
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<FallbackProviderEditor {...baseProps({ onDelete })} />);
    await userEvent.click(screen.getByRole('button', { name: /delete/i }));
    expect(onDelete).toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('skips onDelete when confirm is denied', async () => {
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<FallbackProviderEditor {...baseProps({ onDelete })} />);
    await userEvent.click(screen.getByRole('button', { name: /delete/i }));
    expect(onDelete).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });
});

describe('FallbackProviderEditor — Fetch models', () => {
  it('renders a "Fetch models" button', () => {
    render(<FallbackProviderEditor {...baseProps()} />);
    expect(screen.getByRole('button', { name: /fetch models/i })).toBeInTheDocument();
  });

  it('disables Fetch while the row is dirty with a "Save first" tooltip', () => {
    render(<FallbackProviderEditor {...baseProps({ dirty: true })} />);
    const btn = screen.getByRole('button', { name: /fetch models/i });
    expect(btn).toBeDisabled();
    expect(btn.getAttribute('title')).toMatch(/save first/i);
  });

  it('shows a "Connected ✓ (N models)" chip on success', async () => {
    const fetchModels = vi.fn(async () => ({ models: ['m1', 'm2', 'm3'] }));
    render(<FallbackProviderEditor {...baseProps({ fetchModels })} />);
    await userEvent.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/Connected ✓ \(3 models\)/)).toBeInTheDocument();
    });
  });

  it('shows a red error chip with the upstream message on failure', async () => {
    const fetchModels = vi.fn(async () => {
      throw new Error('401 unauthorized');
    });
    render(<FallbackProviderEditor {...baseProps({ fetchModels })} />);
    await userEvent.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/401 unauthorized/i)).toBeInTheDocument();
    });
  });

  it('wires fetched models as datalist options on the Model field', async () => {
    const fetchModels = vi.fn(async () => ({ models: ['alpha', 'beta'] }));
    const { container } = render(<FallbackProviderEditor {...baseProps({ fetchModels })} />);
    await userEvent.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      const list = container.querySelector('datalist[data-testid="fallback-model-datalist"]');
      expect(list).not.toBeNull();
      const options = list!.querySelectorAll('option');
      const values = Array.from(options).map(o => o.getAttribute('value'));
      expect(values).toContain('alpha');
      expect(values).toContain('beta');
    });
  });
});
