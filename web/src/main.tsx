import React from 'react';
import { createRoot } from 'react-dom/client';
import './styles/theme.css';
import App from './App';
import { initI18n } from './i18n';

const rootElem = document.getElementById('root');
if (!rootElem) {
  throw new Error('hermind: #root element missing');
}

initI18n()
  .catch((err) => console.error('i18n init failed:', err))
  .finally(() => {
    createRoot(rootElem).render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
  });
