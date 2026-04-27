import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SkillsSection from './SkillsSection';
import type { ConfigSection as ConfigSectionT } from '../../../api/schemas';

const skillsSection: ConfigSectionT = {
  key: 'skills',
  label: 'Skills',
  summary: 'Enable or disable installed skills.',
  group_id: 'skills',
  shape: 'map',
  fields: [
    { name: 'disabled', label: 'Disabled skills', kind: 'multiselect', enum: [] },
    { name: 'auto_extract', label: 'Auto-extract', kind: 'bool', default: false },
    { name: 'inject_count', label: 'Inject count', kind: 'int', default: 3 },
    { name: 'generation_half_life', label: 'Half-life', kind: 'int', default: 5 },
  ],
};

function mockSkillsApi(skills: Array<{ name: string; description?: string; enabled: boolean }>) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response(JSON.stringify({ skills }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('SkillsSection', () => {
  it('renders Globals panel + Skills panel headings', async () => {
    mockSkillsApi([]);
    render(
      <SkillsSection
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        config={{}}
      />,
    );
    expect(screen.getByText('Global settings')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(/no skills installed/i)).toBeInTheDocument());
  });

  it('renders each skill as a row with name, description, and switch', async () => {
    mockSkillsApi([
      { name: 'alpha', description: 'Alpha description', enabled: true },
      { name: 'beta', description: 'Beta description', enabled: true },
      { name: 'gamma', description: 'Gamma description', enabled: false },
    ]);
    render(
      <SkillsSection
        section={skillsSection}
        value={{ disabled: ['gamma'] }}
        originalValue={{ disabled: ['gamma'] }}
        onField={vi.fn()}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('alpha'));
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('Alpha description')).toBeInTheDocument();
    expect(screen.getByText('gamma')).toBeInTheDocument();

    const switches = screen.getAllByRole('switch');
    const skillSwitches = switches.filter(s => /^Enable /.test(s.getAttribute('aria-label') ?? ''));
    expect(skillSwitches).toHaveLength(3);
    expect(skillSwitches[0].getAttribute('aria-checked')).toBe('true'); // alpha
    expect(skillSwitches[2].getAttribute('aria-checked')).toBe('false'); // gamma
  });

  it('toggling an enabled skill OFF appends its name to disabled', async () => {
    mockSkillsApi([
      { name: 'alpha', description: '', enabled: true },
      { name: 'beta', description: '', enabled: true },
    ]);
    const onField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{ disabled: [] }}
        originalValue={{ disabled: [] }}
        onField={onField}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('alpha'));
    fireEvent.click(screen.getByRole('switch', { name: 'Enable alpha' }));
    expect(onField).toHaveBeenCalledWith('disabled', ['alpha']);
  });

  it('toggling a disabled skill ON removes its name from disabled', async () => {
    mockSkillsApi([
      { name: 'alpha', description: '', enabled: false },
      { name: 'beta', description: '', enabled: false },
    ]);
    const onField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{ disabled: ['alpha', 'beta'] }}
        originalValue={{ disabled: ['alpha', 'beta'] }}
        onField={onField}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('alpha'));
    fireEvent.click(screen.getByRole('switch', { name: 'Enable alpha' }));
    expect(onField).toHaveBeenCalledWith('disabled', ['beta']);
  });

  it('renders ghost row for disabled name not in API response', async () => {
    mockSkillsApi([
      { name: 'alpha', description: 'Alpha', enabled: true },
    ]);
    render(
      <SkillsSection
        section={skillsSection}
        value={{ disabled: ['phantom'] }}
        originalValue={{ disabled: ['phantom'] }}
        onField={vi.fn()}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('phantom'));
    expect(screen.getByText(/missing/i)).toBeInTheDocument();
  });

  it('shows error + retry on fetch failure, retry refetches', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response('{"error":"boom"}', { status: 500, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response('{"skills":[]}', { status: 200, headers: { 'Content-Type': 'application/json' } }));
    render(
      <SkillsSection
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText(/failed to load skills/i));
    fireEvent.click(screen.getByText('Retry'));
    await waitFor(() => screen.getByText(/no skills installed/i));
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it('clicking the auto_extract switch dispatches onField with a boolean', async () => {
    mockSkillsApi([]);
    const onField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{ auto_extract: false }}
        originalValue={{ auto_extract: false }}
        onField={onField}
        config={{}}
      />,
    );
    const autoExtract = screen.getByRole('switch', { name: /auto-extract/i });
    fireEvent.click(autoExtract);
    expect(onField).toHaveBeenCalledWith('auto_extract', true);
  });

  it('inject_count input dispatches onField with the raw string', async () => {
    mockSkillsApi([]);
    const onField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{ inject_count: 3 }}
        originalValue={{ inject_count: 3 }}
        onField={onField}
        config={{}}
      />,
    );
    const input = document.querySelector('#skills-inject-count') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '7' } });
    expect(onField).toHaveBeenCalledWith('inject_count', '7');
  });
});
