window.addEventListener('message', (event) => {
  if (event.data?.type === 'NEW_BROWSER_EXTENSION_CONNECTION') {
    const parts = event.data.apiKey.split('|');
    if (parts.length === 2) {
      chrome.storage.sync.set({ apiBase: parts[0], apiKey: parts[1] }, () => {
        chrome.runtime.sendMessage({ action: 'connectionUpdated' });
      });
    }
  }
});