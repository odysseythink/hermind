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

// Fetch /api/status early so the tab title reflects the instance even
// before React mounts. Failures are silent — the tab keeps the default.
fetch('/api/status')
  .then((r) => r.json())
  .then((s: { instance_root?: string }) => setTitleFromInstance(s.instance_root ?? ''))
  .catch(() => { /* ignore */ });

initI18n()
  .catch((err) => console.error('i18n init failed:', err))
  .finally(() => {
    createRoot(rootElem).render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
  });
