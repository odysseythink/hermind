import React from 'react';
import { createRoot } from 'react-dom/client';
import './styles/theme.css';
import 'katex/dist/katex.min.css';
import App from './App';
import { initI18n } from './i18n';

const rootElem = document.getElementById('root');
if (!rootElem) {
  throw new Error('hermind: #root element missing');
}

function setTitleFromInstance(instanceRoot: string) {
  if (!instanceRoot) return;
  const parts = instanceRoot.split('/').filter(Boolean);
  const label = parts.length >= 2
    ? `${parts[parts.length - 2]}/${parts[parts.length - 1]}`
    : instanceRoot;
  document.title = `hermind — ${label}`;
}

// Mount React immediately without waiting for i18n, then initialize i18n
// in the background. This unblocks SSE connections and reduces FCP.
console.time('React mount');
createRoot(rootElem).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
console.timeEnd('React mount');

console.time('i18n init');
// Initialize i18n in background (non-blocking)
initI18n()
  .then(() => console.timeEnd('i18n init'))
  .catch((err) => console.error('i18n init failed:', err));

// Fetch /api/status early so the tab title reflects the instance even
// before React mounts. Failures are silent — the tab keeps the default.
fetch('/api/status')
  .then((r) => r.json())
  .then((s: { instance_root?: string }) => setTitleFromInstance(s.instance_root ?? ''))
  .catch(() => { /* ignore */ });
