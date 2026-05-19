import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ToolDetailFallback from './ToolDetailFallback';
import type { ConfigField } from '../../../api/schemas';

const baseProps = {
  name: 'test_tool',
  enabled: true,
  onToggle: vi.fn(),
  onSectionField: vi.fn(),
};

describe('ToolDetailFallback', () => {
  it('renders name and toggle', () => {
    render(<ToolDetailFallback {...baseProps} description="A test tool" />);
    expect(screen.getByText('test_tool')).toBeInTheDocument();
    expect(screen.getByText('A test tool')).toBeInTheDocument();
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('dispatches onToggle when switch is clicked', () => {
    render(<ToolDetailFallback {...baseProps} />);
    fireEvent.click(screen.getByRole('switch'));
    expect(baseProps.onToggle).toHaveBeenCalledWith(false);
  });

  it('renders "no settings" when schema is empty', () => {
    render(<ToolDetailFallback {...baseProps} settings_schema={[]} />);
    expect(screen.getByText(/此工具暂无配置项/)).toBeInTheDocument();
  });

  it('renders string input and dispatches on change', () => {
    const schema: ConfigField[] = [
      { name: 'host', label: 'Host', kind: 'string', help: 'Server host' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    expect(screen.getByText('Host')).toBeInTheDocument();
    const input = screen.getByLabelText('Host');
    fireEvent.change(input, { target: { value: 'example.com' } });
    expect(baseProps.onSectionField).toHaveBeenCalledWith('tools', 'settings', {
      test_tool: { host: 'example.com' },
    });
  });

  it('renders bool switch and dispatches on change', () => {
    const schema: ConfigField[] = [
      { name: 'verbose', label: 'Verbose', kind: 'bool' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    const sw = screen.getByRole('switch', { name: 'Verbose' });
    fireEvent.click(sw);
    expect(baseProps.onSectionField).toHaveBeenCalledWith('tools', 'settings', {
      test_tool: { verbose: true },
    });
  });

  it('groups fields by group property', () => {
    const schema: ConfigField[] = [
      { name: 'a', label: 'A', kind: 'string', group: 'Network' },
      { name: 'b', label: 'B', kind: 'string', group: 'Auth' },
    ];
    render(<ToolDetailFallback {...baseProps} settings_schema={schema} config={{}} />);
    expect(screen.getByText('Network')).toBeInTheDocument();
    expect(screen.getByText('Auth')).toBeInTheDocument();
  });

  it('hides fields when visible_when predicate is not met', () => {
    const schema: ConfigField[] = [
      { name: 'use_proxy', label: 'Use proxy', kind: 'bool' },
      { name: 'proxy_url', label: 'Proxy URL', kind: 'string', visible_when: { field: 'use_proxy', equals: true } },
    ];
    render(
      <ToolDetailFallback
        {...baseProps}
        settings_schema={schema}
        config={{ tools: { settings: { test_tool: { use_proxy: false } } } }}
      />,
    );
    expect(screen.getByText('Use proxy')).toBeInTheDocument();
    expect(screen.queryByText('Proxy URL')).not.toBeInTheDocument();
  });

  it('shows hidden field when visible_when predicate is met', () => {
    const schema: ConfigField[] = [
      { name: 'use_proxy', label: 'Use proxy', kind: 'bool' },
      { name: 'proxy_url', label: 'Proxy URL', kind: 'string', visible_when: { field: 'use_proxy', equals: true } },
    ];
    render(
      <ToolDetailFallback
        {...baseProps}
        settings_schema={schema}
        config={{ tools: { settings: { test_tool: { use_proxy: true } } } }}
      />,
    );
    expect(screen.getByText('Proxy URL')).toBeInTheDocument();
  });
});
