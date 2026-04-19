import React from 'react';
import { createRoot } from 'react-dom/client';
import './styles/theme.css';
import App from './App';

const rootElem = document.getElementById('root');
if (!rootElem) {
  throw new Error('hermind: #root element missing');
}
createRoot(rootElem).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
