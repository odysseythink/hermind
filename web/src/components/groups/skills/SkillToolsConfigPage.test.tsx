import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import SkillToolsConfigPage from './SkillToolsConfigPage';
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

function mockApis(
  skills: Array<{ name: string; description?: string; enabled: boolean }>,
  tools: Array<{ name: string; description?: string; toolset?: string; enabled: boolean; settings_schema?: unknown[] }>,
) {
  return vi.spyOn(globalThis, 'fetch').mockImplementation((input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    if (url.includes('/api/skills')) {
      return Promise.resolve(
        new Response(JSON.stringify({ skills }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );
    }
    if (url.includes('/api/tools')) {
      return Promise.resolve(
        new Response(JSON.stringify({ tools }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );
    }
    return Promise.resolve(new Response('{}', { status: 404 }));
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('SkillToolsConfigPage', () => {
  it('renders BrowserControlConfig for browser_control tool', async () => {
    mockApis(
      [],
      [{ name: 'browser_control', description: 'Browser control', toolset: 'browser', enabled: true, settings_schema: [] }],
    );
    render(
      <SkillToolsConfigPage
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        onSectionField={vi.fn()}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('browser_control'));
    fireEvent.click(screen.getByText('browser_control'));
    await waitFor(() => expect(screen.getByTestId('status-unknown')).toBeInTheDocument());
  });

  it('renders ToolDetailFallback for unregistered tools', async () => {
    mockApis(
      [],
      [
        {
          name: 'web_search',
          description: 'Search',
          toolset: 'web',
          enabled: true,
          settings_schema: [{ name: 'api_key', label: 'API Key', kind: 'secret' }],
        },
      ],
    );
    render(
      <SkillToolsConfigPage
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        onSectionField={vi.fn()}
        config={{}}
      />,
    );
    await waitFor(() => screen.getByText('web_search'));
    fireEvent.click(screen.getByText('web_search'));
    await waitFor(() => expect(screen.getByLabelText('API Key')).toBeInTheDocument());
  });
});
