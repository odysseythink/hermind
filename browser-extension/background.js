// Background service worker for Hermind extension
// Handles both: (1) user-initiated page scrape via popup, (2) LLM-driven browser commands via polling

const POLL_INTERVAL_MS = 2000;
const TASK_TIMEOUT_MS = 60000;

// ===== Settings =====

async function getSettings() {
  return chrome.storage.sync.get({
    hermindUrl: 'http://localhost:36265'
  });
}

// ===== Task Polling Loop =====

let pollTimer = null;
let isPolling = false;

function startPolling() {
  if (isPolling) return;
  isPolling = true;
  pollLoop();
}

function stopPolling() {
  isPolling = false;
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
}

async function pollLoop() {
  if (!isPolling) return;

  try {
    const settings = await getSettings();

    const url = settings.hermindUrl.replace(/\/$/, '') + '/api/browser-extension/poll';
    const response = await fetch(url, {
      method: 'GET'
    });

    if (!response.ok) {
      pollTimer = setTimeout(pollLoop, 5000);
      return;
    }

    const task = await response.json();
    if (task && !task.empty && task.id) {
      console.log('[Hermind] Received task:', task.action, task.id);
      await executeTask(task);
      // Poll immediately after completing a task
      pollTimer = setTimeout(pollLoop, 500);
      return;
    }
  } catch (err) {
    console.error('[Hermind] Poll error:', err.message);
  }

  pollTimer = setTimeout(pollLoop, POLL_INTERVAL_MS);
}

// ===== Task Execution =====

async function executeTask(task) {
  const result = { task_id: task.id, success: false, content: '', error: '', url: '', title: '' };

  try {
    switch (task.action) {
      case 'navigate':
        await executeNavigate(task, result);
        break;
      case 'click':
        await executeClick(task, result);
        break;
      case 'fill':
        await executeFill(task, result);
        break;
      case 'scroll':
        await executeScroll(task, result);
        break;
      case 'screenshot':
        await executeScreenshot(task, result);
        break;
      case 'extract_text':
        await executeExtractText(task, result);
        break;
      case 'extract_html':
        await executeExtractHTML(task, result);
        break;
      case 'wait':
        await executeWait(task, result);
        break;
      case 'switch_tab':
        await executeSwitchTab(task, result);
        break;
      case 'list_tabs':
        await executeListTabs(task, result);
        break;
      case 'close_tab':
        await executeCloseTab(task, result);
        break;
      default:
        result.error = `Unknown action: ${task.action}`;
    }
  } catch (err) {
    result.error = err.message || String(err);
  }

  await reportResult(result);
}

async function reportResult(result) {
  try {
    const settings = await getSettings();
    const url = settings.hermindUrl.replace(/\/$/, '') + '/api/browser-extension/result';
    await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(result)
    });
  } catch (err) {
    console.error('[Hermind] Failed to report result:', err);
  }
}

// ===== Action Implementations =====

// Always operate on the most recently created/used Hermind-managed tab
// We track our own tab IDs to avoid interfering with user's browsing
const managedTabs = new Set();

async function getTargetTab() {
  // Prefer the most recently added managed tab
  for (const tabId of Array.from(managedTabs).reverse()) {
    try {
      const tab = await chrome.tabs.get(tabId);
      if (tab && !tab.discarded) return tab;
    } catch (e) {
      managedTabs.delete(tabId);
    }
  }
  // Fall back to active tab
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  return tab;
}

async function executeNavigate(task, result) {
  const tab = await chrome.tabs.create({ url: task.url, active: false });
  managedTabs.add(tab.id);
  // Wait for load
  await waitForTabLoad(tab.id, 15000);
  const updated = await chrome.tabs.get(tab.id);
  result.success = true;
  result.url = updated.url;
  result.title = updated.title;
  result.content = `Navigated to ${updated.url} (${updated.title})`;
}

async function executeClick(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: (selector) => {
      const el = document.querySelector(selector);
      if (!el) throw new Error(`Element not found: ${selector}`);
      el.click();
      return { title: document.title, url: location.href };
    },
    args: [task.selector]
  });
  // Brief wait for navigation/change
  await sleep(1000);
  const updated = await chrome.tabs.get(tab.id);
  result.success = true;
  result.url = updated.url;
  result.title = updated.title;
  result.content = `Clicked ${task.selector} on ${updated.url}`;
}

async function executeFill(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: (selector, value) => {
      const el = document.querySelector(selector);
      if (!el) throw new Error(`Element not found: ${selector}`);
      el.focus();
      el.value = value;
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
      return { title: document.title, url: location.href };
    },
    args: [task.selector, task.value]
  });
  result.success = true;
  result.content = `Filled ${task.selector} with "${task.value}"`;
}

async function executeScroll(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  const direction = task.direction || 'down';
  const amount = task.amount || 500;
  await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: (dir, amt) => {
      const y = dir === 'up' ? -amt : amt;
      window.scrollBy(0, y);
      return { scrollY: window.scrollY, title: document.title };
    },
    args: [direction, amount]
  });
  result.success = true;
  result.content = `Scrolled ${direction} by ${amount}px`;
}

async function executeScreenshot(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  // Ensure tab is active for screenshot
  await chrome.tabs.update(tab.id, { active: true });
  await sleep(500);
  const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' });
  result.success = true;
  result.content = dataUrl; // base64 PNG
}

async function executeExtractText(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  const [{ result: text }] = await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: () => {
      const selectors = ['main', 'article', '[role="main"]', '.content', '#content', 'body'];
      for (const sel of selectors) {
        const el = document.querySelector(sel);
        if (el) {
          const t = el.innerText || el.textContent;
          if (t && t.trim().length > 100) return { text: t.trim(), url: location.href, title: document.title };
        }
      }
      return { text: (document.body.innerText || '').trim(), url: location.href, title: document.title };
    }
  });
  result.success = true;
  result.content = text.text;
  result.url = text.url;
  result.title = text.title;
}

async function executeExtractHTML(task, result) {
  const tab = await getTargetTab();
  if (!tab) throw new Error('No target tab available');
  const [{ result: html }] = await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: () => ({ html: document.documentElement.outerHTML, url: location.href, title: document.title })
  });
  result.success = true;
  result.content = html.html;
  result.url = html.url;
  result.title = html.title;
}

async function executeWait(task, result) {
  const ms = task.duration_ms || 2000;
  await sleep(ms);
  const tab = await getTargetTab();
  if (tab) {
    result.url = tab.url;
    result.title = tab.title;
  }
  result.success = true;
  result.content = `Waited ${ms}ms`;
}

async function executeSwitchTab(task, result) {
  const tabs = await chrome.tabs.query({ currentWindow: true });
  const idx = task.tab_index || 0;
  if (idx < 0 || idx >= tabs.length) throw new Error(`Invalid tab index: ${idx}`);
  await chrome.tabs.update(tabs[idx].id, { active: true });
  result.success = true;
  result.url = tabs[idx].url;
  result.title = tabs[idx].title;
  result.content = `Switched to tab ${idx}: ${tabs[idx].title}`;
}

async function executeListTabs(task, result) {
  const tabs = await chrome.tabs.query({ currentWindow: true });
  const list = tabs.map((t, i) => `${i}: ${t.title} (${t.url})`).join('\n');
  result.success = true;
  result.content = list;
}

async function executeCloseTab(task, result) {
  const tabs = await chrome.tabs.query({ currentWindow: true });
  const idx = task.tab_index;
  if (idx !== undefined && idx >= 0 && idx < tabs.length) {
    managedTabs.delete(tabs[idx].id);
    await chrome.tabs.remove(tabs[idx].id);
    result.success = true;
    result.content = `Closed tab ${idx}`;
  } else {
    // Close active managed tab
    const tab = await getTargetTab();
    if (tab) {
      managedTabs.delete(tab.id);
      await chrome.tabs.remove(tab.id);
      result.success = true;
      result.content = `Closed tab: ${tab.title}`;
    } else {
      throw new Error('No tab to close');
    }
  }
}

// ===== Helpers =====

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function waitForTabLoad(tabId, timeoutMs) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Tab load timeout')), timeoutMs);
    chrome.tabs.onUpdated.addListener(function listener(updatedTabId, changeInfo) {
      if (updatedTabId === tabId && changeInfo.status === 'complete') {
        clearTimeout(timer);
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      }
    });
  });
}

// ===== Message Handlers (from popup/options) =====

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.type === 'SCRAPE_PAGE') {
    handleScrape(request.data)
      .then(r => sendResponse({ success: true, data: r }))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }
  if (request.type === 'CHECK_CONNECTION') {
    checkConnection()
      .then(r => sendResponse({ success: true, data: r }))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }
  if (request.type === 'GET_STATUS') {
    sendResponse({ polling: isPolling, managedTabs: Array.from(managedTabs) });
    return true;
  }
});

async function handleScrape(data) {
  const settings = await getSettings();
  const url = settings.hermindUrl.replace(/\/$/, '') + '/api/browser-extension/scrape';
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data)
  });
  if (!response.ok) {
    throw new Error(`Server returned ${response.status}`);
  }
  return await response.json();
}

async function checkConnection() {
  const settings = await getSettings();
  const url = settings.hermindUrl.replace(/\/$/, '') + '/api/browser-extension/check';
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Server returned ${response.status}`);
  return await response.json();
}

// ===== Lifecycle =====

chrome.runtime.onStartup.addListener(startPolling);
chrome.runtime.onInstalled.addListener(startPolling);

// Also start immediately when service worker wakes up
startPolling();
