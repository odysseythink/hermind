import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import AdvancedSidebar from './AdvancedSidebar';

function baseProps(
  overrides: Partial<ComponentProps<typeof AdvancedSidebar>> = {},
): ComponentProps<typeof AdvancedSidebar> {
  return {
    activeSubKey: null,
    onSelectScalar: vi.fn(),
    cronJobs: [],
    dirtyCronIndices: new Set<number>(),
    activeCronIndex: null,
    onSelectCron: vi.fn(),
    onAddCronJob: vi.fn(),
    onMoveCron: vi.fn(),
    ...overrides,
  };
}

describe('AdvancedSidebar', () => {
  it('renders "Browser" row', () => {
    render(<AdvancedSidebar {...baseProps()} />);
    expect(screen.getByRole('button', { name: /^browser$/i })).toBeInTheDocument();
  });

  it('clicking Browser calls onSelectScalar("browser")', async () => {
    const onSelectScalar = vi.fn();
    render(<AdvancedSidebar {...baseProps({ onSelectScalar })} />);
    await userEvent.click(screen.getByRole('button', { name: /^browser$/i }));
    expect(onSelectScalar).toHaveBeenCalledWith('browser');
  });

  it('Browser row is active when activeSubKey is "browser"', () => {
    render(<AdvancedSidebar {...baseProps({ activeSubKey: 'browser' })} />);
    const btn = screen.getByRole('button', { name: /^browser$/i });
    expect(btn.className).toMatch(/active/);
  });

  it('shows empty state when cronJobs is empty', () => {
    render(<AdvancedSidebar {...baseProps({ cronJobs: [] })} />);
    expect(screen.getByText(/no cron jobs configured/i)).toBeInTheDocument();
  });

  it('renders each cron job with its name and schedule', () => {
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'daily-report', schedule: '0 9 * * *' },
            { name: 'hourly-check', schedule: 'every 1h' },
          ],
        })}
      />,
    );
    expect(screen.getByText('daily-report')).toBeInTheDocument();
    expect(screen.getByText('0 9 * * *')).toBeInTheDocument();
    expect(screen.getByText('hourly-check')).toBeInTheDocument();
    expect(screen.getByText('every 1h')).toBeInTheDocument();
  });

  it('renders #N position badge for each job', () => {
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'job-a', schedule: '* * * * *' },
            { name: 'job-b', schedule: '* * * * *' },
          ],
        })}
      />,
    );
    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
  });

  it('renders dirty dot for dirty indices', () => {
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'job-a', schedule: '* * * * *' },
            { name: 'job-b', schedule: '* * * * *' },
          ],
          dirtyCronIndices: new Set([1]),
        })}
      />,
    );
    const dots = document.querySelectorAll('[title="Unsaved changes"]');
    expect(dots.length).toBe(1);
  });

  it('renders "(unnamed)" when name is empty', () => {
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [{ name: '', schedule: 'every 5m' }],
        })}
      />,
    );
    expect(screen.getByText('(unnamed)')).toBeInTheDocument();
  });

  it('"Add cron job" button calls onAddCronJob', async () => {
    const onAddCronJob = vi.fn();
    render(<AdvancedSidebar {...baseProps({ onAddCronJob })} />);
    await userEvent.click(screen.getByRole('button', { name: /add cron job/i }));
    expect(onAddCronJob).toHaveBeenCalled();
  });

  it('move-up disabled at index 0, move-down disabled at last', () => {
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'a', schedule: '* * * * *' },
            { name: 'b', schedule: '* * * * *' },
          ],
        })}
      />,
    );
    const upBtns = screen.getAllByRole('button', { name: /move up/i });
    const downBtns = screen.getAllByRole('button', { name: /move down/i });
    expect(upBtns[0]).toBeDisabled();
    expect(downBtns[0]).not.toBeDisabled();
    expect(upBtns[1]).not.toBeDisabled();
    expect(downBtns[1]).toBeDisabled();
  });

  it('calls onSelectCron with the correct index when a row is clicked', async () => {
    const onSelectCron = vi.fn();
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'job-a', schedule: '* * * * *' },
            { name: 'job-b', schedule: '* * * * *' },
          ],
          onSelectCron,
        })}
      />,
    );
    await userEvent.click(screen.getByText('job-b'));
    expect(onSelectCron).toHaveBeenCalledWith(1);
  });

  it('calls onMoveCron(i, direction) on arrow buttons', async () => {
    const onMoveCron = vi.fn();
    render(
      <AdvancedSidebar
        {...baseProps({
          cronJobs: [
            { name: 'a', schedule: '* * * * *' },
            { name: 'b', schedule: '* * * * *' },
          ],
          onMoveCron,
        })}
      />,
    );
    await userEvent.click(screen.getAllByRole('button', { name: /move down/i })[0]);
    expect(onMoveCron).toHaveBeenCalledWith(0, 'down');
    await userEvent.click(screen.getAllByRole('button', { name: /move up/i })[1]);
    expect(onMoveCron).toHaveBeenCalledWith(1, 'up');
  });
});
