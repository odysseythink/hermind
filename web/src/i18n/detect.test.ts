import { describe, it, expect, beforeEach, vi } from 'vitest';
import { detectInitialLanguage, SUPPORTED_LANGUAGES, STORAGE_KEY } from './detect';

describe('detectInitialLanguage', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('exposes the supported language list', () => {
    expect(SUPPORTED_LANGUAGES).toEqual(['en', 'zh-CN']);
  });

  it('prefers localStorage when it holds a supported language', () => {
    localStorage.setItem(STORAGE_KEY, 'zh-CN');
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('en-US');
    expect(detectInitialLanguage()).toBe('zh-CN');
  });

  it('ignores localStorage when it holds an unsupported value', () => {
    localStorage.setItem(STORAGE_KEY, 'fr');
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('en-US');
    expect(detectInitialLanguage()).toBe('en');
  });

  it('falls back to navigator.language for zh-*', () => {
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('zh-TW');
    expect(detectInitialLanguage()).toBe('zh-CN');
  });

  it('falls back to navigator.language for en-*', () => {
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('en-GB');
    expect(detectInitialLanguage()).toBe('en');
  });

  it('falls back to en when navigator.language is neither en nor zh', () => {
    vi.spyOn(navigator, 'language', 'get').mockReturnValue('ja-JP');
    expect(detectInitialLanguage()).toBe('en');
  });
});
