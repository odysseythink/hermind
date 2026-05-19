// Popup script for Hermind extension

(async function() {
  const els = {
    pageInfo: document.getElementById('pageInfo'),
    sendBtn: document.getElementById('sendBtn'),
    btnText: document.querySelector('.btn-text'),
    spinner: document.querySelector('.spinner'),
    message: document.getElementById('message'),
    statusDot: document.getElementById('statusDot'),
    optionsLink: document.getElementById('optionsLink')
  };

  let currentTab = null;
  let pageData = null;

  // Get current tab
  async function getCurrentTab() {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    return tab;
  }

  // Extract page content
  async function extractContent(format) {
    if (!currentTab) return null;
    try {
      const response = await chrome.tabs.sendMessage(currentTab.id, {
        type: 'GET_PAGE_CONTENT',
        format
      });
      return response.data;
    } catch (err) {
      // Content script might not be injected yet, inject it
      await chrome.scripting.executeScript({
        target: { tabId: currentTab.id },
        files: ['content.js']
      });
      // Try again
      const response = await chrome.tabs.sendMessage(currentTab.id, {
        type: 'GET_PAGE_CONTENT',
        format
      });
      return response.data;
    }
  }

  // Check connection to Hermind
  async function checkConnection() {
    try {
      const response = await chrome.runtime.sendMessage({ type: 'CHECK_CONNECTION' });
      if (response.success) {
        els.statusDot.classList.add('connected');
        return true;
      }
      throw new Error(response.error);
    } catch (err) {
      els.statusDot.classList.remove('connected');
      showMessage(err.message, 'error');
      return false;
    }
  }

  // Show message
  function showMessage(text, type) {
    els.message.textContent = text;
    els.message.className = 'message ' + type;
    if (type === 'success') {
      setTimeout(() => {
        els.message.textContent = '';
        els.message.className = 'message';
      }, 5000);
    }
  }

  // Update page info display
  function updatePageInfo(tab, data) {
    const title = data?.title || tab.title || 'Unknown';
    const url = tab.url || '';
    els.pageInfo.innerHTML = `
      <div class="page-title">${escapeHtml(title)}</div>
      <div class="page-url">${escapeHtml(url)}</div>
    `;
    els.sendBtn.disabled = false;
  }

  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  // Set loading state
  function setLoading(loading) {
    els.sendBtn.disabled = loading;
    els.btnText.classList.toggle('hidden', loading);
    els.spinner.classList.toggle('hidden', !loading);
  }

  // Send content to Hermind
  async function sendToHermind() {
    const format = document.querySelector('input[name="format"]:checked').value;

    setLoading(true);
    showMessage('Extracting page content...', '');

    try {
      const data = await extractContent(format);
      if (!data || !data.content) {
        throw new Error('Failed to extract page content');
      }

      showMessage('Sending to Hermind...', '');
      const response = await chrome.runtime.sendMessage({
        type: 'SCRAPE_PAGE',
        data
      });

      if (response.success) {
        showMessage(`Sent! Document ID: ${response.data.id || 'saved'}`, 'success');
      } else {
        throw new Error(response.error || 'Unknown error');
      }
    } catch (err) {
      showMessage(err.message, 'error');
    } finally {
      setLoading(false);
    }
  }

  // Open options page
  function openOptions() {
    chrome.runtime.openOptionsPage();
  }

  // Event listeners
  els.sendBtn.addEventListener('click', sendToHermind);
  els.optionsLink.addEventListener('click', (e) => {
    e.preventDefault();
    openOptions();
  });

  // Initialize
  async function init() {
    currentTab = await getCurrentTab();

    // Check for restricted URLs
    if (!currentTab.url || currentTab.url.startsWith('chrome://') || currentTab.url.startsWith('edge://')) {
      els.pageInfo.innerHTML = '<div class="page-title">Cannot access this page</div>';
      els.sendBtn.disabled = true;
      return;
    }

    const connected = await checkConnection();
    if (connected) {
      // Pre-extract content
      const data = await extractContent('text');
      pageData = data;
      updatePageInfo(currentTab, data);
    } else {
      updatePageInfo(currentTab, { title: currentTab.title });
      els.sendBtn.disabled = true;
    }
  }

  init();
})();
