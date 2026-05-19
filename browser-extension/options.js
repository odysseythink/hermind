// Options page script for Hermind extension

(function() {
  const form = document.getElementById('settingsForm');
  const hermindUrlInput = document.getElementById('hermindUrl');
  const apiKeyInput = document.getElementById('apiKey');
  const toggleKeyBtn = document.getElementById('toggleKey');
  const testBtn = document.getElementById('testBtn');
  const messageEl = document.getElementById('message');

  // Load saved settings
  async function loadSettings() {
    const result = await chrome.storage.sync.get({
      hermindUrl: 'http://localhost:8080',
      apiKey: ''
    });
    hermindUrlInput.value = result.hermindUrl;
    apiKeyInput.value = result.apiKey;
  }

  // Show message
  function showMessage(text, type) {
    messageEl.textContent = text;
    messageEl.className = 'message show ' + type;
    setTimeout(() => {
      messageEl.className = 'message';
    }, 5000);
  }

  // Save settings
  async function saveSettings(e) {
    e.preventDefault();
    const hermindUrl = hermindUrlInput.value.trim().replace(/\/$/, '');
    const apiKey = apiKeyInput.value.trim();

    await chrome.storage.sync.set({ hermindUrl, apiKey });
    showMessage('Settings saved successfully!', 'success');
  }

  // Test connection
  async function testConnection() {
    testBtn.disabled = true;
    testBtn.textContent = 'Testing...';

    try {
      const response = await chrome.runtime.sendMessage({ type: 'CHECK_CONNECTION' });
      if (response.success) {
        showMessage(`Connected! Hermind version: ${response.data.version || 'unknown'}`, 'success');
      } else {
        throw new Error(response.error || 'Connection failed');
      }
    } catch (err) {
      showMessage(err.message, 'error');
    } finally {
      testBtn.disabled = false;
      testBtn.textContent = 'Test Connection';
    }
  }

  // Toggle API key visibility
  function toggleKeyVisibility() {
    const type = apiKeyInput.type === 'password' ? 'text' : 'password';
    apiKeyInput.type = type;
    toggleKeyBtn.textContent = type === 'password' ? '👁' : '🙈';
  }

  // Event listeners
  form.addEventListener('submit', saveSettings);
  testBtn.addEventListener('click', testConnection);
  toggleKeyBtn.addEventListener('click', toggleKeyVisibility);

  // Init
  loadSettings();
})();
