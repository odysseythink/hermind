import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
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

describe('ConfigSection — datalist_source on a string field', () => {
  const modelSection: ConfigSectionT = {
    key: 'model',
    label: 'Default model',
    group_id: 'models',
    shape: 'scalar',
    fields: [
      {
        name: 'model',
        label: 'Model',
        kind: 'string',
        required: true,
        datalist_source: { section: 'providers', field: 'model' },
      },
    ],
  };

  it('passes suggestions collected from a keyed_map section to TextInput', () => {
    const { container } = render(
      <ConfigSection
        section={modelSection}
        value={{ model: '' }}
        originalValue={{ model: '' }}
        onFieldChange={() => {}}
        config={{
          providers: {
            anthropic_main: { provider: 'anthropic', model: 'claude-opus-4-7' },
            openai_bot: { provider: 'openai', model: 'gpt-4o' },
          },
        }}
      />,
    );
    const options = container.querySelectorAll('datalist option');
    expect(options).toHaveLength(2);
    const values = Array.from(options).map(o => o.getAttribute('value')).sort();
    expect(values).toEqual(['claude-opus-4-7', 'gpt-4o']);
  });

  it('passes suggestions collected from a list section to TextInput', () => {
    const sectionPointingAtList: ConfigSectionT = {
      ...modelSection,
      fields: [
        {
          ...modelSection.fields[0],
          datalist_source: { section: 'fallback_providers', field: 'model' },
        },
      ],
    };
    const { container } = render(
      <ConfigSection
        section={sectionPointingAtList}
        value={{ model: '' }}
        originalValue={{ model: '' }}
        onFieldChange={() => {}}
        config={{
          fallback_providers: [
            { provider: 'anthropic', model: 'claude-sonnet-4-6' },
            { provider: 'openai', model: 'gpt-4o-mini' },
          ],
        }}
      />,
    );
    const values = Array.from(
      container.querySelectorAll('datalist option'),
    ).map(o => o.getAttribute('value')).sort();
    expect(values).toEqual(['claude-sonnet-4-6', 'gpt-4o-mini']);
  });

  it('deduplicates identical suggestions', () => {
    const { container } = render(
      <ConfigSection
        section={modelSection}
        value={{ model: '' }}
        originalValue={{ model: '' }}
        onFieldChange={() => {}}
        config={{
          providers: {
            a: { provider: 'anthropic', model: 'claude-opus-4-7' },
            b: { provider: 'anthropic', model: 'claude-opus-4-7' },
          },
        }}
      />,
    );
    const options = container.querySelectorAll('datalist option');
    expect(options).toHaveLength(1);
  });

  it('skips blank and non-string values', () => {
    const { container } = render(
      <ConfigSection
        section={modelSection}
        value={{ model: '' }}
        originalValue={{ model: '' }}
        onFieldChange={() => {}}
        config={{
          providers: {
            a: { provider: 'anthropic', model: '' },
            b: { provider: 'openai', model: null },
            c: { provider: 'openai', model: 'gpt-4o' },
          },
        }}
      />,
    );
    const values = Array.from(
      container.querySelectorAll('datalist option'),
    ).map(o => o.getAttribute('value'));
    expect(values).toEqual(['gpt-4o']);
  });

  it('omits the datalist entirely when the source section is absent', () => {
    const { container } = render(
      <ConfigSection
        section={modelSection}
        value={{ model: '' }}
        originalValue={{ model: '' }}
        onFieldChange={() => {}}
        config={{}}
      />,
    );
    expect(container.querySelector('datalist')).toBeNull();
  });
});

const nested: ConfigSectionT = {
  key: 'memory',
  label: 'Memory',
  group_id: 'memory',
  fields: [
    { name: 'provider', label: 'Provider', kind: 'enum', enum: ['', 'honcho'] },
    {
      name: 'honcho.api_key', label: 'Honcho API key', kind: 'secret',
      visible_when: { field: 'provider', equals: 'honcho' },
    },
  ],
};

describe('ConfigSection — dotted field names', () => {
  it('renders a nested value and dispatches the dotted name on edit', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();

    function Host() {
      const [value, setValue] = useState<Record<string, unknown>>({
        provider: 'honcho',
        honcho: { api_key: 'orig-key' },
      });
      return (
        <ConfigSection
          section={nested}
          value={value}
          originalValue={value}
          onFieldChange={(name, v) => {
            setValue(prev => {
              // Host mirrors the reducer behavior for the test.
              if (!name.includes('.')) return { ...prev, [name]: v };
              const [head, ...rest] = name.split('.');
              const inner = (prev[head] as Record<string, unknown>) ?? {};
              return { ...prev, [head]: { ...inner, [rest.join('.')]: v } };
            });
            onFieldChange(name, v);
          }}
        />
      );
    }

    render(<Host />);
    const input = screen.getByLabelText(/honcho api key/i) as HTMLInputElement;
    expect(input.value).toBe('orig-key');
    await user.clear(input);
    await user.type(input, 'new-key');
    const last = onFieldChange.mock.calls[onFieldChange.mock.calls.length - 1];
    expect(last[0]).toBe('honcho.api_key');
    expect(last[1]).toBe('new-key');
  });

  it('hides a dotted-name field when the sibling discriminator does not match', () => {
    render(
      <ConfigSection
        section={nested}
        value={{ provider: '' }}
        originalValue={{ provider: '' }}
        onFieldChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText(/honcho api key/i)).toBeNull();
  });
});

describe('ConfigSection — multiselect dispatch', () => {
  const section: ConfigSectionT = {
    key: 'skills',
    label: 'Skills',
    group_id: 'skills',
    shape: 'map',
    fields: [
      {
        name: 'disabled',
        label: 'Disabled skills',
        kind: 'multiselect',
        enum: ['alpha', 'beta'],
      },
    ],
  };

  it('renders MultiSelectField for kind: multiselect', () => {
    render(
      <ConfigSection
        section={section}
        value={{ disabled: ['alpha'] }}
        originalValue={{ disabled: [] }}
        onFieldChange={vi.fn()}
      />,
    );
    const alpha = screen.getByLabelText('alpha') as HTMLInputElement;
    const beta = screen.getByLabelText('beta') as HTMLInputElement;
    expect(alpha.checked).toBe(true);
    expect(beta.checked).toBe(false);
  });

  it('calls onFieldChange with the new string[] when a checkbox toggles', () => {
    const onFieldChange = vi.fn();
    render(
      <ConfigSection
        section={section}
        value={{ disabled: [] }}
        originalValue={{ disabled: [] }}
        onFieldChange={onFieldChange}
      />,
    );
    fireEvent.click(screen.getByLabelText('beta'));
    expect(onFieldChange).toHaveBeenCalledWith('disabled', ['beta']);
  });
});
