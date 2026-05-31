import { useState, useEffect, useCallback } from 'react';
import BrowserExtensionAPI from '../models/browserExtension';

export function useApiConnection() {
  const [status, setStatus] = useState('disconnected');
  const [apiBase, setApiBase] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [workspaces, setWorkspaces] = useState([]);
  const [logoUrl, setLogoUrl] = useState(null);

  useEffect(() => {
    chrome.storage.sync.get(['apiBase', 'apiKey'], (items) => {
      if (items.apiBase && items.apiKey) {
        setApiBase(items.apiBase);
        setApiKey(items.apiKey);
        checkConnection(items.apiBase, items.apiKey);
      }
    });
  }, []);

  const checkConnection = useCallback(async (base, key) => {
    setStatus('connecting');
    try {
      const api = new BrowserExtensionAPI(base, key);
      const data = await api.check();
      if (data.connected) {
        setStatus('connected');
        setWorkspaces(data.workspaces || []);
        const logo = await api.fetchLogo();
        if (logo) setLogoUrl(logo);
      } else {
        setStatus('error');
      }
    } catch (e) {
      setStatus('error');
    }
  }, []);

  const connect = useCallback((connectionString) => {
    const parts = connectionString.split('|');
    if (parts.length !== 2) return;
    const [base, key] = parts;
    chrome.storage.sync.set({ apiBase: base, apiKey: key }, () => {
      setApiBase(base);
      setApiKey(key);
      checkConnection(base, key);
    });
  }, [checkConnection]);

  const disconnect = useCallback(async () => {
    try {
      const api = new BrowserExtensionAPI(apiBase, apiKey);
      await api.disconnect();
    } catch (e) {}
    chrome.storage.sync.remove(['apiBase', 'apiKey'], () => {
      setApiBase('');
      setApiKey('');
      setStatus('disconnected');
      setWorkspaces([]);
    });
  }, [apiBase, apiKey]);

  return { status, apiBase, apiKey, workspaces, logoUrl, connect, disconnect };
}