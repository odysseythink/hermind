import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ConfigSection from './ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../api/schemas';

const storage: ConfigSectionT = {
  key: 'storage',
  label: 'Storage',
  summary: 'Where hermind keeps data.',
  group_id: 'runtime',
  fields: [
    { name: 'driver', label: 'Driver', kind: 'enum',
      required: true, default: 'sqlite', enum: ['sqlite', 'postgres'] },
    { name: 'sqlite_path', label: 'SQLite path', kind: 'string',
      visible_when: { field: 'driver', equals: 'sqlite' } },
    { name: 'postgres_url', label: 'Postgres URL', kind: 'secret',
      visible_when: { field: 'driver', equals: 'postgres' } },
  ],
};

describe('ConfigSection', () => {
  it('renders fields whose visible_when matches', () => {
    render(
      <ConfigSection
        section={storage}
        value={{ driver: 'sqlite', sqlite_path: '/var/db/x' }}
        originalValue={{ driver: 'sqlite', sqlite_path: '/var/db/x' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/driver/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/postgres url/i)).not.toBeInTheDocument();
  });

  it('flips visible fields when the discriminator changes', () => {
    const { rerender } = render(
      <ConfigSection
        section={storage}
        value={{ driver: 'sqlite' }}
        originalValue={{ driver: 'sqlite' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/sqlite path/i)).toBeInTheDocument();

    rerender(
      <ConfigSection
        section={storage}
        value={{ driver: 'postgres' }}
        originalValue={{ driver: 'sqlite' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/sqlite path/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/postgres url/i)).toBeInTheDocument();
  });

  it('dispatches onFieldChange with field name + value', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();

    // Wrapper mirrors the real parent-reducer flow: when ConfigSection
    // dispatches a change, the parent updates state and re-renders with
    // the new value. ConfigSection itself is stateless — the reducer is
    // the single source of truth.
    function Host() {
      const [value, setValue] = useState<Record<string, unknown>>({
        driver: 'sqlite',
        sqlite_path: '',
      });
      return (
        <ConfigSection
          section={storage}
          value={value}
          originalValue={{ driver: 'sqlite', sqlite_path: '' }}
          onFieldChange={(name, v) => {
            setValue(prev => ({ ...prev, [name]: v }));
            onFieldChange(name, v);
          }}
        />
      );
    }

    render(<Host />);
    const input = screen.getByLabelText(/sqlite path/i);
    await user.type(input, '/tmp/x.sqlite');

    const calls = onFieldChange.mock.calls;
    expect(calls.length).toBeGreaterThan(0);
    expect(calls[calls.length - 1][0]).toBe('sqlite_path');
    expect(calls[calls.length - 1][1]).toBe('/tmp/x.sqlite');
  });

  it('disables the Show button on secret fields with an explanatory tooltip', () => {
    render(
      <ConfigSection
        section={storage}
        value={{ driver: 'postgres', postgres_url: '' }}
        originalValue={{ driver: 'postgres', postgres_url: '' }}
        onFieldChange={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /show/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute('title', 'Reveal not supported for this field (stage 2)');
  });
});

const tracing: ConfigSectionT = {
  key: 'tracing',
  label: 'Tracing',
  group_id: 'observability',
  fields: [
    { name: 'enabled', label: 'Enabled', kind: 'bool' },
    {
      name: 'file',
      label: 'File',
      kind: 'string',
      visible_when: { field: 'enabled', equals: true },
    },
  ],
};

describe('ConfigSection isVisible — bool predicate round-trip', () => {
  it('shows the File field when enabled is the backend bool true', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: true }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/^File$/)).toBeInTheDocument();
  });

  it('shows the File field after the BoolToggle stores "true" (string)', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: 'true' }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.getByLabelText(/^File$/)).toBeInTheDocument();
  });

  it('hides the File field when enabled is "false" (string)', () => {
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: 'false' }}
        originalValue={{ enabled: true }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/^File$/)).toBeNull();
  });

  it('dispatches the edited enabled value as a string', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();
    render(
      <ConfigSection
        section={tracing}
        value={{ enabled: true }}
        originalValue={{ enabled: true }}
        onFieldChange={onFieldChange}
      />,
    );
    await user.click(screen.getByLabelText(/Enabled/));
    expect(onFieldChange).toHaveBeenCalledWith('enabled', 'false');
  });
});
