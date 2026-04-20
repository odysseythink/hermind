import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import Sidebar from './Sidebar';
import type { ConfigSection, SchemaDescriptor } from '../../api/schemas';

const descriptors: SchemaDescriptor[] = [] as SchemaDescriptor[];

function baseProps(
  overrides: Partial<ComponentProps<typeof Sidebar>> = {},
): ComponentProps<typeof Sidebar> {
  return {
    activeGroup: null,
    activeSubKey: null,
    expandedGroups: new Set(['gateway']),
    dirtyGroups: new Set(),
    instances: [],
    selectedKey: null,
    descriptors,
    configSections: [],
    dirtyInstanceKeys: new Set<string>(),
    onSelectGroup: vi.fn(),
    onSelectSub: vi.fn(),
    onToggleGroup: vi.fn(),
    onNewInstance: vi.fn(),
    ...overrides,
  };
}

describe('Sidebar', () => {
  it('renders all 7 group labels', () => {
    render(<Sidebar {...baseProps()} />);
    for (const label of [
      'Models',
      'Gateway',
      'Memory',
      'Skills',
      'Runtime',
      'Advanced',
      'Observability',
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it('shows instance children only under Gateway when expanded', () => {
    render(
      <Sidebar
        {...baseProps({
          instances: [{ key: 'feishu', type: 'feishu', enabled: true }],
          expandedGroups: new Set(['gateway']),
        })}
      />,
    );
    expect(screen.getAllByText('feishu').length).toBeGreaterThan(0);
  });

  it('shows "Coming soon" rows under expanded non-Gateway groups', () => {
    render(
      <Sidebar
        {...baseProps({
          expandedGroups: new Set(['gateway', 'models']),
        })}
      />,
    );
    expect(screen.getByText(/coming soon — stage 3 & 4/i)).toBeInTheDocument();
  });

  it('calls onToggleGroup when an arrow is clicked', async () => {
    const onToggleGroup = vi.fn();
    render(<Sidebar {...baseProps({ onToggleGroup })} />);
    await userEvent.click(screen.getAllByRole('button', { name: /toggle models/i })[0]);
    expect(onToggleGroup).toHaveBeenCalledWith('models');
  });

  it('marks Gateway as dirty when dirtyGroups contains gateway', () => {
    render(<Sidebar {...baseProps({ dirtyGroups: new Set(['gateway']) })} />);
    expect(screen.getByTitle(/unsaved/i)).toBeInTheDocument();
  });
});

describe('Sidebar — non-gateway groups', () => {
  const storageSection: ConfigSection = {
    key: 'storage', label: 'Storage', group_id: 'runtime', fields: [],
  };

  it('renders registered sections inside expanded non-gateway groups', () => {
    render(
      <Sidebar
        activeGroup="runtime"
        activeSubKey={null}
        expandedGroups={new Set(['runtime'])}
        dirtyGroups={new Set()}
        instances={[]}
        selectedKey={null}
        descriptors={[]}
        configSections={[storageSection]}
        dirtyInstanceKeys={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: /storage/i })).toBeInTheDocument();
  });

  it('shows "Coming soon" in groups with no registered sections', () => {
    render(
      <Sidebar
        activeGroup={null}
        activeSubKey={null}
        expandedGroups={new Set(['memory'])}
        dirtyGroups={new Set()}
        instances={[]}
        selectedKey={null}
        descriptors={[]}
        configSections={[]}
        dirtyInstanceKeys={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
      />,
    );
    expect(screen.getByText(/coming soon — stage 5/i)).toBeInTheDocument();
  });
});
