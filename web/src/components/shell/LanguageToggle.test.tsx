import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from 'i18next';
import LanguageToggle from './LanguageToggle';
import { STORAGE_KEY } from '../../i18n/detect';

describe('LanguageToggle', () => {
  beforeEach(async () => {
    localStorage.clear();
    await i18n.changeLanguage('en');
  });

  it('marks the current language as pressed', () => {
    render(<LanguageToggle />);
    expect(screen.getByRole('button', { name: 'EN' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '中' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('switches to zh-CN and persists', async () => {
    const user = userEvent.setup();
    render(<LanguageToggle />);
    await user.click(screen.getByRole('button', { name: '中' }));
    expect(i18n.language).toBe('zh-CN');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('zh-CN');
    expect(screen.getByRole('button', { name: '中' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('switches back to en', async () => {
    const user = userEvent.setup();
    await i18n.changeLanguage('zh-CN');
    render(<LanguageToggle />);
    await user.click(screen.getByRole('button', { name: 'EN' }));
    expect(i18n.language).toBe('en');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('en');
  });

  it('exposes an aria-label from translations', () => {
    render(<LanguageToggle />);
    expect(screen.getByRole('group')).toHaveAttribute('aria-label', 'Language');
  });
});
