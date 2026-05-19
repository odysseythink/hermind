// Content script for extracting page content

(function() {
  'use strict';

  function extractText() {
    // Try to get main content first
    const selectors = [
      'main',
      'article',
      '[role="main"]',
      '.content',
      '#content',
      '.post-content',
      '.entry-content',
      'body'
    ];

    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) {
        const text = el.innerText || el.textContent;
        if (text && text.trim().length > 100) {
          return text.trim();
        }
      }
    }

    return document.body.innerText || document.body.textContent || '';
  }

  function extractHtml() {
    return document.documentElement.innerHTML;
  }

  // Listen for scrape requests from popup
  chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
    if (request.type === 'GET_PAGE_CONTENT') {
      const format = request.format || 'text';
      const title = document.title || '';
      const url = window.location.href;

      let content;
      if (format === 'html') {
        content = extractHtml();
      } else {
        content = extractText();
      }

      sendResponse({
        success: true,
        data: {
          url,
          title,
          content,
          format,
          timestamp: Math.floor(Date.now() / 1000)
        }
      });
    }
    return true;
  });
})();
