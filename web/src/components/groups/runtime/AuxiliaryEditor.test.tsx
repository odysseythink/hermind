import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AuxiliaryEditor from './AuxiliaryEditor';
import type { ConfigSection } from '../../../api/schemas';

const auxSection: ConfigSection = {
  key: 'auxiliary',
  label: 'Auxiliary',
  group_id: 'runtime',
  fields: [
    { name: 'provider', label: 'Provider', kind: 'enum', enum: ['anthropic', 'qwen'] },
    { name: 'base_url', label: 'Base URL', kind: 'string' },
    { name: 'api_key', label: 'API key', kind: 'secret' },
    { name: 'model', label: 'Model', kind: 'string' },
  ],
};

const inst = { provider: 'qwen', base_url: '', api_key: '', model: 'qwen-plus' };

describe('AuxiliaryEditor', () => {
  function baseProps(overrides: any = {}) {
    return {
      section: auxSection,
      value: inst,
      originalValue: inst,
      dirty: false,
      onField: () => {},
      fetchModels: async () => ({ models: [] }),
      testConnection: async () => ({ ok: true, latency_ms: 0 }),
      ...overrides,
    };
  }

  it('renders the model field as a <select> seeded with the current value', () => {
    render(<AuxiliaryEditor {...baseProps()} />);
    const select = screen.getByLabelText(/^Model/) as HTMLSelectElement;
    expect(select.tagName).toBe('SELECT');
    const options = Array.from(select.querySelectorAll('option')).map(o => o.value);
    expect(options).toContain('qwen-plus');
  });

  it('expands the model dropdown after a successful fetch', async () => {
    const user = userEvent.setup();
    const fetchModels = vi.fn().mockResolvedValue({ models: ['qwen-plus', 'qwen-max'] });
    render(<AuxiliaryEditor {...baseProps({ fetchModels })} />);
    await user.click(screen.getByRole('button', { name: /fetch models/i }));
    await waitFor(() => {
      expect(screen.getByText(/connected/i)).toBeInTheDocument();
    });
    const select = screen.getByLabelText(/^Model/) as HTMLSelectElement;
    const options = Array.from(select.querySelectorAll('option')).map(o => o.value);
    expect(options).toContain('qwen-plus');
    expect(options).toContain('qwen-max');
  });

  it('shows latency chip after a successful Test click', async () => {
    const user = userEvent.setup();
    const testConnection = vi.fn().mockResolvedValue({ ok: true, latency_ms: 88 });
    render(<AuxiliaryEditor {...baseProps({ testConnection })} />);
    await user.click(screen.getByRole('button', { name: /^test$/i }));
    await waitFor(() => {
      expect(screen.getByText(/test passed/i)).toBeInTheDocument();
    });
    expect(testConnection).toHaveBeenCalledOnce();
    expect(screen.getByText(/88ms/)).toBeInTheDocument();
  });

  it('shows error chip when Test rejects', async () => {
    const user = userEvent.setup();
    const testConnection = vi.fn().mockRejectedValue(new Error('auth failed'));
    render(<AuxiliaryEditor {...baseProps({ testConnection })} />);
    await user.click(screen.getByRole('button', { name: /^test$/i }));
    await waitFor(() => {
      expect(screen.getByText(/auth failed/i)).toBeInTheDocument();
    });
  });

  it('disables both buttons while dirty', () => {
    render(<AuxiliaryEditor {...baseProps({ dirty: true })} />);
    expect(screen.getByRole('button', { name: /fetch models/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /^test$/i })).toBeDisabled();
  });
});
