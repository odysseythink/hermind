import '@testing-library/jest-dom/vitest';
import { afterEach, beforeAll } from 'vitest';
import { cleanup } from '@testing-library/react';
import { initI18n } from '../i18n';

beforeAll(async () => {
  await initI18n();
});

afterEach(() => {
  cleanup();
});
