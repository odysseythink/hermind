import { describe, it, expect, beforeEach } from 'vitest';
import i18n from 'i18next';
import { initI18n, setLanguage } from './index';
import { STORAGE_KEY } from './detect';

describe('initI18n', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('initializes i18next with en + zh-CN resources and ui + descriptors namespaces', async () => {
    await initI18n();
    expect(i18n.hasResourceBundle('en', 'ui')).toBe(true);
    expect(i18n.hasResourceBundle('en', 'descriptors')).toBe(true);
    expect(i18n.hasResourceBundle('zh-CN', 'ui')).toBe(true);
    expect(i18n.hasResourceBundle('zh-CN', 'descriptors')).toBe(true);
  });

  it('disables keySeparator so dotted keys are literal', async () => {
    await initI18n();
    expect(i18n.options.keySeparator).toBe(false);
  });

  it('falls back to en', async () => {
    await initI18n();
    expect(i18n.options.fallbackLng).toEqual(['en']);
  });

  it('sets document.documentElement.lang', async () => {
    await initI18n();
    expect(document.documentElement.lang).toBe(i18n.language);
  });
});

describe('setLanguage', () => {
  beforeEach(async () => {
    localStorage.clear();
    await initI18n();
  });

  it('changes i18n language, persists to localStorage, and updates <html lang>', () => {
    setLanguage('zh-CN');
    expect(i18n.language).toBe('zh-CN');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('zh-CN');
    expect(document.documentElement.lang).toBe('zh-CN');
  });
});
