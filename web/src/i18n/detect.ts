export const SUPPORTED_LANGUAGES = ['en', 'zh-CN'] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];
export const STORAGE_KEY = 'hermind.lang';

function isSupported(v: string | null | undefined): v is SupportedLanguage {
  return !!v && (SUPPORTED_LANGUAGES as readonly string[]).includes(v);
}

export function detectInitialLanguage(): SupportedLanguage {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (isSupported(stored)) return stored;
  } catch {
    /* SSR / private-browsing safe */
  }

  const nav = typeof navigator !== 'undefined' ? navigator.language || '' : '';
  if (nav.toLowerCase().startsWith('zh')) return 'zh-CN';

  return 'en';
}
