import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import ComingSoonPanel from './ComingSoonPanel';
import type { Config } from '../../api/schemas';

const cfg: Config = {};

describe('ComingSoonPanel', () => {
  it('renders the group label as a heading', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByRole('heading', { name: /models/i })).toBeInTheDocument();
  });

  it('displays the planned stage string', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/stage 3 & 4/i)).toBeInTheDocument();
  });

  it('renders the group bullets', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/default model selection/i)).toBeInTheDocument();
    expect(screen.getByText(/fallback providers/i)).toBeInTheDocument();
  });

  it('renders the summary for the group', () => {
    render(
      <ComingSoonPanel
        group="memory"
        config={{ memory: { enabled: true, retain_db: {} } } as unknown as Config}
      />,
    );
    expect(screen.getByText(/retain_db/i)).toBeInTheDocument();
    expect(screen.getByText(/yes/i)).toBeInTheDocument();
  });

  it('renders the Edit via CLI escape hatch', () => {
    render(<ComingSoonPanel group="models" config={cfg} />);
    expect(screen.getByText(/hermind config --web/i)).toBeInTheDocument();
  });
});
