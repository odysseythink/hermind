# Hermind Browser Extension

A Chrome/Edge browser extension that lets you send web page content — including **logged-in pages** — directly to your Hermind instance.

## Why?

The built-in `web_scrape_site` tool uses a headless browser that has no access to your logged-in sessions. This extension solves that by extracting content from **your actual browser**, where you're already authenticated.

## Features

- 🔐 **Works with logged-in pages** — captures content from any page you can see
- 📝 **Text or HTML** — choose the format that works best
- ⚡ **One-click send** — click the icon, pick a format, done
- 🔗 **Direct integration** — content appears in Hermind via the `browser_extension_read` tool

## Installation

### 1. Configure Hermind

Add to your Hermind `config.yaml`:

```yaml
browser_extension:
  enabled: true
  api_key: "your-random-secret-key-here"  # generate with: openssl rand -hex 32
```

Restart Hermind.

### 2. Load the Extension (Developer Mode)

**Chrome / Edge:**
1. Open `chrome://extensions` (or `edge://extensions`)
2. Enable **Developer mode** (toggle in top-right)
3. Click **Load unpacked**
4. Select the `browser-extension/` folder

### 3. Configure the Extension

1. Click the extension icon → **Settings**
2. Enter your Hermind server URL (e.g. `http://localhost:8080`)
3. Paste the same `api_key` from your `config.yaml`
4. Click **Test Connection** to verify

## Usage

1. Navigate to any web page (even one that requires login)
2. Click the **Hermind** icon in your browser toolbar
3. Choose **Text** (clean innerText) or **HTML** (raw HTML)
4. Click **Send to Hermind**
5. In Hermind, ask the AI about the page:
   > "Summarize the page I just sent from the browser"
   
   The AI will use the `browser_extension_read` tool to access it.

## Architecture

```
┌─────────────┐     extract content      ┌─────────────┐
│   Browser   │ ───────────────────────> │   Content   │
│   (logged   │   (content script)       │   Script    │
│    in)      │                          └──────┬──────┘
└─────────────┘                                 │
       │                                        │
       │ click send                             │
       ▼                                        ▼
┌─────────────┐     POST /api/browser-    ┌─────────────┐
│    Popup    │ ───────────────────────>  │   Hermind   │
│    (UI)     │   X-Extension-Key header  │    Server   │
└─────────────┘                           └──────┬──────┘
                                                 │
                                                 │ save to
                                                 ▼
                                          ┌─────────────┐
                                          │  browser-   │
                                          │ extension/  │
                                          │  {id}.md    │
                                          └─────────────┘
```

## Files

| File | Purpose |
|------|---------|
| `manifest.json` | Extension manifest (V3) |
| `background.js` | Service worker — API communication |
| `content.js` | Content script — page extraction |
| `popup.html/js/css` | Extension popup UI |
| `options.html/js/css` | Settings page |

## API Endpoints (Hermind)

- `GET /api/browser-extension/check` — Verify connection
- `POST /api/browser-extension/scrape` — Receive page content

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Invalid API Key" | Make sure the key in extension settings matches `config.yaml` |
| "Browser extension is not configured" | Add `browser_extension.api_key` to `config.yaml` |
| Cannot access `chrome://` pages | Extension cannot run on browser internal pages (by design) |
| Content not extracted | Try refreshing the page first, then click the extension |

## Security Notes

- The API key is stored in the browser's sync storage (encrypted by the browser)
- Only pages you explicitly choose to send are transmitted
- The extension requests minimal permissions: `activeTab`, `storage`, `scripting`
