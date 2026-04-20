import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProviderEditor from './ProviderEditor';
import type { ConfigSection } from '../../../api/schemas';

const providersSection: ConfigSection = {
  key: 'providers',
  label: 'Providers',
  group_id: 'models',
  shape: 'keyed_map',
  fields: [
    { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
      enum: ['anthropic', 'openai'] },
    { name: 'base_url', label: 'Base URL', kind: 'string' },
    { name: 'api_key', label: 'API key', kind: 'secret', required: true },
    { name: 'model', label: 'Default model for this provider', kind: 'string' },
  ],
};

const inst = {
  provider: 'anthropic',
  base_url: 'https://api.anthropic.com',
  api_key: '',
  model: 'anthropic/claude-opus-4-7',
};

describe('ProviderEditor', () => {
  function baseProps(overrides: any = {}) {
    return {
      sectionKey: 'providers',
      instanceKey: 'anthropic_main',
      section: providersSection,
      value: inst,
      originalValue: inst,
      dirty: false,
      onField: () => {},
      onDelete: () => {},
      fetchModels: async () => ({ models: [] }),
      ...overrides,
    };
  }

  it('renders all four fields', () => {
    render(<ProviderEditor {...baseProps()} />);
    expect(screen.getByLabelText(/provider type/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/base url/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/api key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/default model for this provider/i)).toBeInTheDocument();
  });

  it('dispatches onField with the correct instance key when a field changes', async () => {
    const user = userEvent.setup();
    const onField = vi.fn();
    // Stateful Host: controlled inputs need prop updates to reflect typing;
    // thread the field value back through `value` so userEvent.type observes
    // an up-to-date DOM value between keystrokes.
    function Host() {
      const [value, setValue] = useState<Record<string, unknown>>(inst);
      return (
        <ProviderEditor
          {...baseProps({
            value,
            onField: (k: string, field: string, v: unknown) => {
              setValue(prev => ({ ...prev, [field]: v }));
              onField(k, field, v);
            },
          })}
        />
      );
    }
    render(<Host />);
    const input = screen.getByLabelText(/base url/i);
    await user.clear(input);
    await user.type(input, 'https://new');
    const calls = onField.mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    const last = calls[calls.length - 1];
    expect(last[0]).toBe('anthropic_main');
    expect(last[1]).toBe('base_url');
    expect(last[2]).toBe('https://new');
  });

  it('disables the Fetch models button when the instance is dirty', () => {
    render(<ProviderEditor {...baseProps({ dirty: true })} />);
    const btn = screen.getByRole('button', { name: /fetch models/i });
    expect(btn).toBeDisabled();
  });

  it('populates the datalist from a successful fetch', async () => {
    const user = userEvent.setup();
    const fetchModels = vi.fn().mockResolvedValue({ models: ['claude-opus-4-7', 'claude-sonnet-4-6'] });
    render(<ProviderEditor {...baseProps({ fetchModels })} />);
    await user.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/connected/i)).toBeInTheDocument();
    });
    const datalist = screen.getByTestId('provider-model-datalist');
    const options = Array.from(datalist.querySelectorAll('option')).map(o => o.getAttribute('value'));
    expect(options).toEqual(['claude-opus-4-7', 'claude-sonnet-4-6']);
  });

  it('shows an error chip when fetchModels rejects', async () => {
    const user = userEvent.setup();
    const fetchModels = vi.fn().mockRejectedValue(new Error('401 unauthorized'));
    render(<ProviderEditor {...baseProps({ fetchModels })} />);
    await user.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/401 unauthorized/i)).toBeInTheDocument();
    });
  });

  it('prompts confirm and fires onDelete when Delete is clicked', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<ProviderEditor {...baseProps({ onDelete })} />);
    await user.click(screen.getByRole('button', { name: /delete/i }));
    expect(confirmSpy).toHaveBeenCalled();
    expect(onDelete).toHaveBeenCalled();
    confirmSpy.mockRestore();
  });
});
