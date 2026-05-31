const API_BASE_KEY = 'apiBase';
const API_KEY_KEY = 'apiKey';

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: 'save-selected',
    title: 'Save selected to Hermind',
    contexts: ['selection'],
  });
  chrome.contextMenus.create({
    id: 'save-page',
    title: 'Save entire page to Hermind',
    contexts: ['page'],
  });
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;

  if (info.menuItemId === 'save-selected') {
    await uploadContent(apiBase, apiKey, info.selectionText, tab.title, tab.url);
  } else if (info.menuItemId === 'save-page') {
    const text = await getPageContent(tab.id);
    await uploadContent(apiBase, apiKey, text, tab.title, tab.url);
  }
});

async function getPageContent(tabId) {
  try {
    const [{ result }] = await chrome.scripting.executeScript({
      target: { tabId },
      func: () => document.body.innerText,
    });
    return result || '';
  } catch (e) {
    return '';
  }
}

async function uploadContent(apiBase, apiKey, text, title, url) {
  try {
    const resp = await fetch(`${apiBase}/browser-extension/upload-content`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ textContent: text, metadata: { title, url } }),
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '✅' });
      setTimeout(() => chrome.action.setBadgeText({ text: '' }), 2000);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}

chrome.alarms.create('syncWorkspaces', { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name !== 'syncWorkspaces') return;
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;
  try {
    const resp = await fetch(`${apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${apiKey}` },
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '' });
      chrome.action.setTitle({ title: 'Hermind - Connected' });
    } else if (resp.status === 403) {
      await chrome.storage.sync.remove([API_BASE_KEY, API_KEY_KEY]);
      chrome.action.setBadgeText({ text: '❌' });
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
});