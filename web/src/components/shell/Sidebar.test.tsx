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
    providerInstances: [],
    dirtyProviderKeys: new Set<string>(),
    fallbackProviders: [],
    dirtyFallbackIndices: new Set<number>(),
    onSelectGroup: vi.fn(),
    onSelectSub: vi.fn(),
    onToggleGroup: vi.fn(),
    onNewInstance: vi.fn(),
    onNewProvider: vi.fn(),
    onAddFallback: vi.fn(),
    onMoveFallback: vi.fn(),
    onReorderFallback: vi.fn(),
    cronJobs: [],
    dirtyCronIndices: new Set<number>(),
    onAddCronJob: vi.fn(),
    onMoveCron: vi.fn(),
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
          expandedGroups: new Set(['gateway', 'memory']),
        })}
      />,
    );
    expect(screen.getByText(/coming soon — stage 5/i)).toBeInTheDocument();
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
        providerInstances={[]}
        dirtyProviderKeys={new Set()}
        fallbackProviders={[]}
        dirtyFallbackIndices={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
        onNewProvider={() => {}}
        onAddFallback={() => {}}
        onMoveFallback={() => {}}
        onReorderFallback={() => {}}
        cronJobs={[]}
        dirtyCronIndices={new Set()}
        onAddCronJob={() => {}}
        onMoveCron={() => {}}
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
        providerInstances={[]}
        dirtyProviderKeys={new Set()}
        fallbackProviders={[]}
        dirtyFallbackIndices={new Set()}
        onSelectGroup={() => {}}
        onSelectSub={() => {}}
        onToggleGroup={() => {}}
        onNewInstance={() => {}}
        onNewProvider={() => {}}
        onAddFallback={() => {}}
        onMoveFallback={() => {}}
        onReorderFallback={() => {}}
        cronJobs={[]}
        dirtyCronIndices={new Set()}
        onAddCronJob={() => {}}
        onMoveCron={() => {}}
      />,
    );
    expect(screen.getByText(/coming soon — stage 5/i)).toBeInTheDocument();
  });
});

describe('Sidebar — models group', () => {
  it('renders ModelsSidebar for the models group (not SectionList)', () => {
    const providersSection: ConfigSection = {
      key: 'providers', label: 'Providers', group_id: 'models', shape: 'keyed_map',
      fields: [
        { name: 'provider', label: 'Provider type', kind: 'enum', required: true,
          enum: ['anthropic'] },
      ],
    };
    const modelSection: ConfigSection = {
      key: 'model', label: 'Default model', group_id: 'models', shape: 'scalar',
      fields: [{ name: 'model', label: 'Model', kind: 'string' }],
    };
    render(
      <Sidebar
        {...baseProps({
          expandedGroups: new Set(['models']),
          configSections: [modelSection, providersSection],
          providerInstances: [{ key: 'anthropic_main', type: 'anthropic' }],
        })}
      />,
    );
    expect(screen.getByText('Default model')).toBeInTheDocument();
    expect(screen.getByText('Providers')).toBeInTheDocument();
    expect(screen.getByText('anthropic_main')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /new provider/i })).toBeInTheDocument();
  });

  it('renders Fallback Providers rows for the models group', () => {
    render(
      <Sidebar
        {...baseProps({
          expandedGroups: new Set(['models']),
          fallbackProviders: [{ provider: 'anthropic' }, { provider: 'openai' }],
        })}
      />,
    );
    expect(screen.getByText('Fallback Providers')).toBeInTheDocument();
    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add fallback/i })).toBeInTheDocument();
  });
});
