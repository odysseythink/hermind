import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import i18n from 'i18next';
import { useDescriptorT } from './useDescriptorT';

describe('useDescriptorT', () => {
  beforeEach(async () => {
    // removeResourceBundle() clears any production-seeded data so the test's
    // minimal fixture bundles below are the ONLY source — otherwise a
    // populated locale file could hit a key this test expects to be missing.
    i18n.removeResourceBundle('en', 'descriptors');
    i18n.removeResourceBundle('zh-CN', 'descriptors');
    i18n.addResourceBundle('en', 'descriptors', {
      'browser.label':                        'Browser',
      'browser.summary':                      'Browser automation provider.',
      'browser.fields.provider.label':        'Provider',
      'browser.fields.provider.help':         'Backend for browser automation.',
      'browser.fields.provider.enum.browserbase': 'Browserbase',
      'groups.runtime':                       'Runtime',
    }, true, true);
    i18n.addResourceBundle('zh-CN', 'descriptors', {
      'browser.label':                 '浏览器',
      'browser.fields.provider.label': 'Provider',
      'groups.runtime':                '运行时',
    }, true, true);
    await i18n.changeLanguage('en');
  });

  it('returns translated section label when key hits', () => {
    const { result } = renderHook(() => useDescriptorT());
    expect(result.current.sectionLabel('browser', 'FALLBACK')).toBe('Browser');
  });

  it('returns fallback when section label key is missing', () => {
    const { result } = renderHook(() => useDescriptorT());
    expect(result.current.sectionLabel('unknown_section', 'English fallback')).toBe('English fallback');
  });

  it('returns fallback when help is missing even if label hit', async () => {
    await i18n.changeLanguage('zh-CN');
    const { result } = renderHook(() => useDescriptorT());
    expect(result.current.fieldHelp('browser', 'provider', 'BACKEND FALLBACK')).toBe('BACKEND FALLBACK');
  });

  it('returns enum translation when present, fallback otherwise', () => {
    const { result } = renderHook(() => useDescriptorT());
    expect(
      result.current.enumValue('browser', 'provider', 'browserbase', 'BROWSERBASE-RAW'),
    ).toBe('Browserbase');
    expect(
      result.current.enumValue('browser', 'provider', 'nonexistent', 'NX-FALLBACK'),
    ).toBe('NX-FALLBACK');
  });

  it('translates group labels', () => {
    const { result } = renderHook(() => useDescriptorT());
    expect(result.current.groupLabel('runtime', 'Runtime')).toBe('Runtime');
    expect(result.current.groupLabel('missing', 'Default Label')).toBe('Default Label');
  });

  it('re-renders after changeLanguage', async () => {
    const { result, rerender } = renderHook(() => useDescriptorT());
    expect(result.current.sectionLabel('browser', 'FB')).toBe('Browser');
    await act(async () => { await i18n.changeLanguage('zh-CN'); });
    rerender();
    expect(result.current.sectionLabel('browser', 'FB')).toBe('浏览器');
  });
});
