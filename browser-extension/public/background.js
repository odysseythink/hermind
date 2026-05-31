const API_BASE_KEY = 'apiBase';
const API_KEY_KEY = 'apiKey';

let workspacesCache = [];

chrome.runtime.onInstalled.addListener(() => {
  rebuildMenus();
});

function rebuildMenus() {
  chrome.contextMenus.removeAll(() => {
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
    chrome.contextMenus.create({
      id: 'sep1',
      type: 'separator',
      contexts: ['selection', 'page'],
    });
    chrome.contextMenus.create({
      id: 'embed-selected-parent',
      title: 'Embed selected to workspace',
      contexts: ['selection'],
    });
    chrome.contextMenus.create({
      id: 'embed-page-parent',
      title: 'Embed entire page to workspace',
      contexts: ['page'],
    });
    for (const ws of workspacesCache) {
      chrome.contextMenus.create({
        parentId: 'embed-selected-parent',
        id: `embed-selected-${ws.id}`,
        title: ws.name,
        contexts: ['selection'],
      });
      chrome.contextMenus.create({
        parentId: 'embed-page-parent',
        id: `embed-page-${ws.id}`,
        title: ws.name,
        contexts: ['page'],
      });
    }
  });
}

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;

  if (info.menuItemId === 'save-selected') {
    await uploadContent(apiBase, apiKey, info.selectionText, tab.title, tab.url);
  } else if (info.menuItemId === 'save-page') {
    const text = await getPageContent(tab.id);
    await uploadContent(apiBase, apiKey, text, tab.title, tab.url);
  } else if (info.menuItemId.startsWith('embed-selected-')) {
    const wsId = parseInt(info.menuItemId.replace('embed-selected-', ''));
    await embedContent(apiBase, apiKey, wsId, info.selectionText, tab.title, tab.url);
  } else if (info.menuItemId.startsWith('embed-page-')) {
    const wsId = parseInt(info.menuItemId.replace('embed-page-', ''));
    const text = await getPageContent(tab.id);
    await embedContent(apiBase, apiKey, wsId, text, tab.title, tab.url);
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
    } else {
      await handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}

async function embedContent(apiBase, apiKey, workspaceId, text, title, url) {
  try {
    const resp = await fetch(`${apiBase}/browser-extension/embed-content`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ workspaceId, textContent: text, metadata: { title, url } }),
    });
    if (resp.ok) {
      chrome.action.setBadgeText({ text: '✅' });
      setTimeout(() => chrome.action.setBadgeText({ text: '' }), 2000);
    } else {
      await handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
}

async function handleApiError(resp) {
  if (resp.status === 403) {
    await chrome.storage.sync.remove([API_BASE_KEY, API_KEY_KEY]);
    workspacesCache = [];
    rebuildMenus();
    chrome.action.setBadgeText({ text: '❌' });
    chrome.action.setTitle({ title: 'Hermind - Disconnected' });
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
      const data = await resp.json();
      workspacesCache = data.workspaces || [];
      rebuildMenus();
      chrome.action.setBadgeText({ text: '' });
      chrome.action.setTitle({ title: 'Hermind - Connected' });
    } else {
      await handleApiError(resp);
    }
  } catch (e) {
    chrome.action.setBadgeText({ text: '⚠️' });
  }
});

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'connectionUpdated') {
    updateWorkspaces();
  }
  sendResponse({ ok: true });
});

async function updateWorkspaces() {
  const { apiBase, apiKey } = await chrome.storage.sync.get([API_BASE_KEY, API_KEY_KEY]);
  if (!apiBase || !apiKey) return;
  try {
    const resp = await fetch(`${apiBase}/browser-extension/check`, {
      headers: { 'Authorization': `Bearer ${apiKey}` },
    });
    if (resp.ok) {
      const data = await resp.json();
      workspacesCache = data.workspaces || [];
      rebuildMenus();
    }
  } catch (e) {
    // ignore
  }
}
