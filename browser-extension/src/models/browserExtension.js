class BrowserExtensionAPI {
  constructor(apiBase, apiKey) {
    this.apiBase = apiBase;
    this.apiKey = apiKey;
  }

  async check() {
    const resp = await fetch(`${this.apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${this.apiKey}` },
    });
    return resp.json();
  }

  async disconnect() {
    const resp = await fetch(`${this.apiBase}/browser-extension/disconnect`, {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${this.apiKey}` },
    });
    return resp.json();
  }

  async fetchLogo() {
    const resp = await fetch(`${this.apiBase}/system/logo`);
    if (!resp.ok) return null;
    const blob = await resp.blob();
    return URL.createObjectURL(blob);
  }
}

export default BrowserExtensionAPI;