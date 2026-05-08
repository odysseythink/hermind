import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

import enUI from '../locales/en/ui.json';
import enDesc from '../locales/en/descriptors.json';
import zhUI from '../locales/zh-CN/ui.json';
import zhDesc from '../locales/zh-CN/descriptors.json';

import { detectInitialLanguage, SUPPORTED_LANGUAGES, STORAGE_KEY } from './detect';

export function initI18n(): Promise<unknown> {
  return i18n
    .use(initReactI18next)
    .init({
      resources: {
        en: { ui: enUI, descriptors: enDesc },
        'zh-CN': { ui: zhUI, descriptors: zhDesc },
      },
      lng: detectInitialLanguage(),
      fallbackLng: 'en',
      supportedLngs: [...SUPPORTED_LANGUAGES],
      defaultNS: 'ui',
      ns: ['ui', 'descriptors'],
      keySeparator: false,
      nsSeparator: ':',
      interpolation: { escapeValue: false },
      returnNull: false,
    })
    .then(() => {
      document.documentElement.lang = i18n.language;
    });
}

export function setLanguage(lang: string): void {
  i18n.changeLanguage(lang);
  localStorage.setItem(STORAGE_KEY, lang);
  document.documentElement.lang = lang;
}

export { STORAGE_KEY };
