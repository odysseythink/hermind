import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ToolsMenu from './ToolsMenu';

describe('ToolsMenu', () => {
  it('does not render when not visible', () => {
    render(<ToolsMenu visible={false} onClose={vi.fn()} onSelect={vi.fn()} suggestions={[]} />);
    expect(screen.queryByText('MCP')).not.toBeInTheDocument();
  });

  it('renders sections and handles selection', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <ToolsMenu
        visible={true}
        onClose={vi.fn()}
        onSelect={onSelect}
        suggestions={['Hello']}
      />
    );
    expect(screen.getByText('MCP')).toBeInTheDocument();
    expect(screen.getByText('Commands')).toBeInTheDocument();

    await user.click(screen.getByText('Hello'));
    expect(onSelect).toHaveBeenCalledWith('Hello', true);
  });
});
