import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GatewaySidebar from './GatewaySidebar';
import type { SchemaDescriptor } from '../../../api/schemas';

const descriptors: SchemaDescriptor[] = [
  { type: 'feishu', display_name: 'Feishu Bot', fields: [] },
  { type: 'dingtalk', display_name: 'DingTalk', fields: [] },
] as unknown as SchemaDescriptor[];

const instances = [
  { key: 'feishu-main', type: 'feishu', enabled: true },
  { key: 'dt-alerts', type: 'dingtalk', enabled: false },
];

describe('GatewaySidebar', () => {
  it('lists every instance', () => {
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText('feishu-main')).toBeInTheDocument();
    expect(screen.getByText('dt-alerts')).toBeInTheDocument();
  });

  it('shows an empty hint when there are no instances', () => {
    render(
      <GatewaySidebar
        instances={[]}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText(/no instances configured/i)).toBeInTheDocument();
  });

  it('calls onSelect when an instance is clicked', async () => {
    const onSelect = vi.fn();
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={onSelect}
        onNewInstance={() => {}}
      />,
    );
    await userEvent.click(screen.getByText('feishu-main'));
    expect(onSelect).toHaveBeenCalledWith('feishu-main');
  });

  it('calls onNewInstance when "+ New" is clicked', async () => {
    const onNewInstance = vi.fn();
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set()}
        onSelect={() => {}}
        onNewInstance={onNewInstance}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /new instance/i }));
    expect(onNewInstance).toHaveBeenCalledTimes(1);
  });

  it('renders a dirty dot next to dirty keys', () => {
    render(
      <GatewaySidebar
        instances={instances}
        selectedKey={null}
        descriptors={descriptors}
        dirtyKeys={new Set(['feishu-main'])}
        onSelect={() => {}}
        onNewInstance={() => {}}
      />,
    );
    // The dirty dot has title="Unsaved changes" in the current Sidebar impl.
    expect(screen.getByTitle(/unsaved changes/i)).toBeInTheDocument();
  });
});
