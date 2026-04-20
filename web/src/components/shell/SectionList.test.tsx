import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SectionList from './SectionList';
import type { ConfigSection } from '../../api/schemas';

const storageSection: ConfigSection = {
  key: 'storage',
  label: 'Storage',
  group_id: 'runtime',
  fields: [],
};

describe('SectionList', () => {
  it('renders the labels of registered sections for the group', () => {
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey={null}
        onSelect={() => {}}
      />,
    );
    expect(screen.getByRole('button', { name: /storage/i })).toBeInTheDocument();
  });

  it('marks the active subKey', () => {
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey="storage"
        onSelect={() => {}}
      />,
    );
    const btn = screen.getByRole('button', { name: /storage/i });
    expect(btn).toHaveAttribute('aria-current', 'true');
  });

  it('dispatches onSelect with the section key', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <SectionList
        group="runtime"
        sections={[storageSection]}
        activeSubKey={null}
        onSelect={onSelect}
      />,
    );
    await user.click(screen.getByRole('button', { name: /storage/i }));
    expect(onSelect).toHaveBeenCalledWith('storage');
  });

  it('falls back to a "Coming soon" row when no sections are registered under the group', () => {
    render(
      <SectionList
        group="memory"
        sections={[]}
        activeSubKey={null}
        onSelect={() => {}}
      />,
    );
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });
});
