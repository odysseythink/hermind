import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import GroupSection from './GroupSection';

describe('GroupSection', () => {
  it('renders the group label', () => {
    render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText(/models/i)).toBeInTheDocument();
  });

  it('shows a right-arrow glyph when collapsed and down-arrow when expanded', () => {
    const { rerender } = render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText('▸')).toBeInTheDocument();
    rerender(
      <GroupSection
        group="models"
        expanded={true}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByText('▾')).toBeInTheDocument();
  });

  it('calls onToggle when the arrow is clicked', async () => {
    const onToggle = vi.fn();
    render(
      <GroupSection
        group="models"
        expanded={false}
        active={false}
        onToggle={onToggle}
        onSelectGroup={() => {}}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /toggle/i }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('calls onSelectGroup when the label is clicked', async () => {
    const onSelectGroup = vi.fn();
    render(
      <GroupSection
        group="memory"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={onSelectGroup}
      />,
    );
    await userEvent.click(screen.getByText(/memory/i));
    expect(onSelectGroup).toHaveBeenCalledWith('memory');
  });

  it('shows children only when expanded', () => {
    const { rerender } = render(
      <GroupSection
        group="gateway"
        expanded={false}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      >
        <div>child content</div>
      </GroupSection>,
    );
    expect(screen.queryByText(/child content/i)).not.toBeInTheDocument();
    rerender(
      <GroupSection
        group="gateway"
        expanded={true}
        active={false}
        onToggle={() => {}}
        onSelectGroup={() => {}}
      >
        <div>child content</div>
      </GroupSection>,
    );
    expect(screen.getByText(/child content/i)).toBeInTheDocument();
  });

  it('shows a dirty dot when dirty=true', () => {
    render(
      <GroupSection
        group="gateway"
        expanded={false}
        active={false}
        dirty
        onToggle={() => {}}
        onSelectGroup={() => {}}
      />,
    );
    expect(screen.getByTitle(/unsaved/i)).toBeInTheDocument();
  });
});
