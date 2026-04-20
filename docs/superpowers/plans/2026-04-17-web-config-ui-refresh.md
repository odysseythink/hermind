# Web Config UI Visual Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the bare-bones `cli/ui/webconfig/web/` assets with a Linear/Vercel-style developer-tool UI that respects OS light/dark preference and uses the TUI amber (`#FFB800`) as the accent, without changing any backend Go code.

**Architecture:** Pure frontend rewrite against the existing `embed.FS` assets. `app.css` grows from 12 lines of flat styling into ~200 lines built on CSS custom properties + a `prefers-color-scheme: dark` override. `index.html` gains a header bar and a structured status element. `app.js` keeps its data-flow logic; the `status()` helper and three render hooks change to emit class names the CSS targets.

**Tech Stack:** Plain HTML, CSS custom properties, `prefers-color-scheme` media query, vanilla JS served from Go's `embed.FS`. No new dependencies, no Go changes, no build step.

---

## File Structure

- Modify: `cli/ui/webconfig/web/index.html` — add `<header class="topbar">`, wrap buttons in `.actions`, restructure `#status` into `dot + msg`. From 18 → ~26 lines.
- Modify: `cli/ui/webconfig/web/app.css` — full rewrite: `:root` custom properties, `@media (prefers-color-scheme: dark)` override, grid layout, sidebar, panel, form rows, inputs, secret field, buttons, status dot, `.flash-error`. From 12 → ~190 lines.
- Modify: `cli/ui/webconfig/web/app.js` — targeted edits only:
  - `status(s, state)` writes to `.msg`, toggles class on `#status` and mirrors on `#topbar-dot`, flashes `.flash-error` on error.
  - `renderForm()` wraps fields in a `<section class="panel">` with a `<header class="panel-header">` containing the section title.
  - Secret (kind 5): wrapper `class="secret-wrap"`, button `class="reveal-btn"`, `Show` / `Hide` text instead of `👁`.
  - List (kind 6): note `class="list-note"` (drop the inline `style.color`).
  - All four `status(...)` call sites pass a state token.
- Untouched: `server.go`, `handlers.go`, `openbrowser.go`, `server_test.go`.

---

## Known Deviation From Spec

The spec's header shows `~/.hermind/config.yaml` in monospace (spec §Layout → Header). No API currently exposes the config path and the spec's non-goals explicitly bar backend changes. The plan renders `<code id="config-path" class="path"></code>` empty, and CSS uses `:empty { display: none }` so the slot collapses invisibly. If we later want the path displayed, a single-line addition to `/api/schema` (or a new `/api/meta`) can populate it — out of scope for this change. Flag this to the user before committing if they prefer we include it.

---

## Task 1: Rewrite `index.html` shell

**Files:**
- Modify: `cli/ui/webconfig/web/index.html`

- [ ] **Step 1: Overwrite `index.html` with the new shell**

Replace the entire file with:

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="color-scheme" content="light dark">
  <title>hermind config</title>
  <link rel="stylesheet" href="/app.css">
</head>
<body>
  <header class="topbar">
    <div class="brand">
      <span class="logo">⬡</span>
      <span class="title">hermind config</span>
    </div>
    <span id="config-path" class="path"></span>
    <span id="topbar-dot" class="dot idle"></span>
  </header>
  <aside id="sections" aria-label="Sections"></aside>
  <main id="form"></main>
  <footer>
    <span id="status" class="status idle" role="status">
      <span class="dot"></span><span class="msg"></span>
    </span>
    <div class="actions">
      <button id="save" class="btn secondary">Save</button>
      <button id="save-exit" class="btn primary">Save &amp; Exit</button>
    </div>
  </footer>
  <script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Verify the file parses**

Run: `python3 -c "import html.parser,sys; p=html.parser.HTMLParser(); p.feed(open('cli/ui/webconfig/web/index.html').read()); print('ok')"`
Expected: `ok`

---

## Task 2: Rewrite `app.css`

**Files:**
- Modify: `cli/ui/webconfig/web/app.css`

- [ ] **Step 1: Overwrite `app.css` with the full stylesheet**

Replace the entire file with:

```css
/* Design tokens — light (default) */
:root {
  --bg: #ffffff;
  --surface: #f8fafc;
  --border: #e5e7eb;
  --text: #111827;
  --muted: #6b7280;
  --accent: #FFB800;
  --accent-fg: #111827;
  --focus: rgba(255, 184, 0, .35);
  --success: #22c55e;
  --error: #ef4444;
  --active-tint: rgba(255, 184, 0, .08);
  --hover-tint: rgba(127, 127, 127, .06);

  --font-sans: ui-sans-serif, system-ui, -apple-system,
               "SF Pro Text", "Segoe UI", "PingFang SC",
               "Microsoft YaHei", sans-serif;
  --font-mono: ui-monospace, "SF Mono", Menlo, "Liberation Mono",
               "Consolas", monospace;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0b0d11;
    --surface: #14171c;
    --border: #2a2f38;
    --text: #e6e8eb;
    --muted: #8892a1;
    --accent: #FFB800;
    --accent-fg: #0b0d11;
    --focus: rgba(255, 184, 0, .5);
    --success: #22c55e;
    --error: #f87171;
  }
}

/* Reset + base */
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; }
body {
  font-family: var(--font-sans);
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  background: var(--bg);
  display: grid;
  grid-template-columns: 240px 1fr;
  grid-template-rows: 48px 1fr 48px;
  grid-template-areas:
    "top  top"
    "side main"
    "foot foot";
  height: 100vh;
}

/* Topbar */
.topbar {
  grid-area: top;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
}
.topbar .brand { display: flex; align-items: center; gap: 8px; }
.topbar .logo { color: var(--accent); font-size: 16px; line-height: 1; }
.topbar .title { font-size: 15px; font-weight: 500; }
.topbar .path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--muted);
  user-select: text;
}
.topbar .path:empty { display: none; }
#topbar-dot { margin-left: auto; }

/* Status dot (shared by topbar + footer) */
.dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: transparent;
  transition: background 120ms ease;
}
.status.unsaved .dot, #topbar-dot.unsaved { background: var(--accent); }
.status.saved   .dot, #topbar-dot.saved   { background: var(--success); }
.status.error   .dot, #topbar-dot.error   { background: var(--error); }

/* Sidebar */
aside#sections {
  grid-area: side;
  background: var(--surface);
  border-right: 1px solid var(--border);
  padding: 16px 12px;
  overflow-y: auto;
}
aside#sections > div {
  position: relative;
  padding: 10px 12px;
  margin: 2px 0;
  border-radius: 8px;
  font-size: 14px;
  color: var(--text);
  cursor: pointer;
  transition: background 120ms ease;
}
aside#sections > div:hover { background: var(--hover-tint); }
aside#sections > div.active {
  background: var(--active-tint);
  font-weight: 500;
}
aside#sections > div.active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 6px;
  bottom: 6px;
  width: 2px;
  background: var(--accent);
  border-radius: 2px;
}

/* Main + panel */
main#form {
  grid-area: main;
  overflow-y: auto;
  padding: 32px 40px;
  background: var(--bg);
}
.panel {
  max-width: 720px;
  margin: 0 auto;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  background: var(--bg);
}
.panel-header { margin-bottom: 16px; }
.panel-header .section-title {
  font-size: 16px;
  font-weight: 500;
  margin: 0;
}

/* Form rows */
.panel label {
  display: block;
  margin: 16px 0;
}
.panel label > .lbl {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 6px;
}
.panel label > .help {
  display: block;
  font-size: 13px;
  color: var(--muted);
  margin-top: 8px;
}

/* Inputs */
.panel input[type=text],
.panel input[type=number],
.panel input[type=password],
.panel select {
  width: 100%;
  height: 38px;
  padding: 0 12px;
  font-size: 14px;
  font-family: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  transition: border-color 120ms ease, box-shadow 120ms ease;
}
.panel input[type=number] { font-variant-numeric: tabular-nums; }
.panel input:focus,
.panel select:focus {
  outline: 2px solid var(--text);
  outline-offset: 2px;
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--focus);
}
.panel input[type=checkbox] {
  width: 16px;
  height: 16px;
  accent-color: var(--accent);
  cursor: pointer;
}

/* Secret field */
.secret-wrap { position: relative; display: block; }
.secret-wrap input { padding-right: 60px; }
.reveal-btn {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: 0;
  padding: 0 4px;
  font: inherit;
  font-size: 12px;
  color: var(--muted);
  cursor: pointer;
}
.reveal-btn:hover { text-decoration: underline; }
.reveal-btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px var(--focus);
  border-radius: 4px;
}

/* List placeholder */
.list-note {
  display: inline-block;
  padding: 8px 12px;
  font-style: italic;
  color: var(--muted);
  background: var(--surface);
  border: 1px dashed var(--border);
  border-radius: 6px;
  font-size: 13px;
}

/* Footer */
footer {
  grid-area: foot;
  display: flex;
  align-items: center;
  padding: 0 16px;
  background: var(--surface);
  border-top: 1px solid var(--border);
  gap: 12px;
  transition: box-shadow 120ms ease;
}
footer.flash-error {
  box-shadow: inset 0 0 0 1px var(--error);
}
#status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--muted);
}
#status .msg { color: var(--text); }
#status.idle .msg { color: transparent; }
.actions { margin-left: auto; display: flex; gap: 8px; }

/* Buttons */
.btn {
  height: 32px;
  padding: 0 14px;
  font-size: 14px;
  font-family: inherit;
  border-radius: 8px;
  cursor: pointer;
  transition: background 120ms ease, filter 120ms ease, box-shadow 120ms ease;
}
.btn.secondary {
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border);
}
.btn.secondary:hover { background: var(--hover-tint); }
.btn.primary {
  background: var(--accent);
  color: var(--accent-fg);
  border: 0;
}
.btn.primary:hover { filter: brightness(0.95); }
.btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px var(--focus);
}
```

- [ ] **Step 2: Sanity-check CSS length and key tokens**

Run: `wc -l cli/ui/webconfig/web/app.css && grep -c -- '--accent' cli/ui/webconfig/web/app.css`
Expected: line count around 280 (≈250–310 range); `--accent` appears at least 10 times (defined twice in `:root` + dark, referenced in accent/active/focus/hover rules).

---

## Task 3: Update `app.js` for status classes, panel header, and field class hooks

**Files:**
- Modify: `cli/ui/webconfig/web/app.js`

All edits preserve the existing fetch / persist / save logic. Four surgical changes.

- [ ] **Step 1: Replace `renderForm()` to wrap fields in a `.panel` with a header**

Find the existing `renderForm` function:

```js
function renderForm() {
  const main = document.getElementById('form');
  main.innerHTML = '';
  schema.filter(f => f.section === currentSection).forEach(f => {
    const wrap = document.createElement('label');
    const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = f.label;
    wrap.appendChild(lbl);
    wrap.appendChild(renderField(f));
    if (f.help) { const h = document.createElement('span'); h.className = 'help'; h.textContent = f.help; wrap.appendChild(h); }
    main.appendChild(wrap);
  });
}
```

Replace it with:

```js
function renderForm() {
  const main = document.getElementById('form');
  main.innerHTML = '';
  const panel = document.createElement('section');
  panel.className = 'panel';
  const header = document.createElement('header');
  header.className = 'panel-header';
  const title = document.createElement('h2');
  title.className = 'section-title';
  title.textContent = currentSection;
  header.appendChild(title);
  panel.appendChild(header);
  schema.filter(f => f.section === currentSection).forEach(f => {
    const wrap = document.createElement('label');
    const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = f.label;
    wrap.appendChild(lbl);
    wrap.appendChild(renderField(f));
    if (f.help) {
      const help = document.createElement('span');
      help.className = 'help';
      help.textContent = f.help;
      wrap.appendChild(help);
    }
    panel.appendChild(wrap);
  });
  main.appendChild(panel);
}
```

- [ ] **Step 2: Update the secret-field renderer to use class hooks and Show/Hide text**

Find the `kind === 5` branch inside `renderField`:

```js
  if (f.kind === 5) {
    const box = document.createElement('span');
    const inp = document.createElement('input'); inp.type = 'password'; inp.value = cur;
    const btn = document.createElement('button'); btn.textContent = '👁'; btn.type = 'button';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {method:'POST', body: JSON.stringify({path: f.path})});
        if (r.ok) { const b = await r.json(); inp.value = b.value; inp.type = 'text'; }
      } else { inp.type = 'password'; }
    };
    inp.onchange = () => persist(f.path, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    return box;
  }
```

Replace it with:

```js
  if (f.kind === 5) {
    const box = document.createElement('span');
    box.className = 'secret-wrap';
    const inp = document.createElement('input'); inp.type = 'password'; inp.value = cur;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'reveal-btn';
    btn.textContent = 'Show';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {method:'POST', body: JSON.stringify({path: f.path})});
        if (r.ok) {
          const b = await r.json();
          inp.value = b.value;
          inp.type = 'text';
          btn.textContent = 'Hide';
        }
      } else {
        inp.type = 'password';
        btn.textContent = 'Show';
      }
    };
    inp.onchange = () => persist(f.path, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    return box;
  }
```

- [ ] **Step 3: Replace the list-field placeholder with a class hook**

Find the `kind === 6` branch:

```js
  if (f.kind === 6) {
    const note = document.createElement('span');
    note.textContent = '(edit via YAML or TUI)';
    note.style.color = '#999';
    return note;
  }
```

Replace it with:

```js
  if (f.kind === 6) {
    const note = document.createElement('span');
    note.className = 'list-note';
    note.textContent = '(edit via YAML or TUI)';
    return note;
  }
```

- [ ] **Step 4: Update the `status()` helper and its call sites**

Find the bottom of the file:

```js
async function persist(path, value) {
  const r = await fetch('/api/config', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path, value})
  });
  if (!r.ok) { status('error: ' + await r.text()); return; }
  values[path] = value;
  status('edited (unsaved)');
}

async function save(exit) {
  const r = await fetch('/api/save', {method:'POST'});
  if (!r.ok) { status('save failed'); return; }
  status('saved — restart hermind to apply');
  if (exit) await fetch('/api/shutdown', {method:'POST'});
}

function status(s) { document.getElementById('status').textContent = s; }
boot();
```

Replace it with:

```js
async function persist(path, value) {
  const r = await fetch('/api/config', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path, value})
  });
  if (!r.ok) { status('error: ' + await r.text(), 'error'); return; }
  values[path] = value;
  status('unsaved changes', 'unsaved');
}

async function save(exit) {
  const r = await fetch('/api/save', {method:'POST'});
  if (!r.ok) { status('save failed', 'error'); return; }
  status('saved — restart hermind to apply', 'saved');
  if (exit) await fetch('/api/shutdown', {method:'POST'});
}

let _flashTimer = null;
function status(s, state) {
  const st = state || 'idle';
  const el = document.getElementById('status');
  el.querySelector('.msg').textContent = s;
  el.className = 'status ' + st;
  const topDot = document.getElementById('topbar-dot');
  if (topDot) topDot.className = 'dot ' + st;
  if (st === 'error') {
    const f = document.querySelector('footer');
    if (f) {
      f.classList.add('flash-error');
      clearTimeout(_flashTimer);
      _flashTimer = setTimeout(() => f.classList.remove('flash-error'), 1000);
    }
  }
}
boot();
```

- [ ] **Step 5: JS syntax check**

Run: `node --check cli/ui/webconfig/web/app.js`
Expected: no output (syntax OK). If `node` is not installed, run `python3 -c "import esprima" 2>/dev/null || echo 'skipped'` and visually review the diff instead.

---

## Task 4: Build, test, and manual smoke

**Files:** none modified — this is pure verification.

- [ ] **Step 1: Build the binary**

Run: `go build ./...`
Expected: exits 0. Webconfig embeds static assets via `//go:embed web/*` in `server.go:16`, so any parse-time issue would surface here.

- [ ] **Step 2: Run the existing webconfig tests**

Run: `go test ./cli/ui/webconfig/...`
Expected: PASS. These tests only exercise API handlers (`server_test.go`), not frontend rendering, so they should be unaffected.

- [ ] **Step 3: Launch the web UI and walk the smoke checklist**

Run: `go build -o bin/hermind ./cmd/hermind && ./bin/hermind config --web`

Visual checks (spec §Testing):
1. Header renders with `⬡ hermind config`, the topbar dot appears on the right but is hidden while idle. Sidebar active item shows the amber 2px left strip + tinted fill. Main column is a bordered panel capped at ~720px.
2. Toggle macOS appearance (System Settings → Appearance → Light / Dark, or `osascript -e 'tell app "System Events" to tell appearance preferences to set dark mode to not dark mode'`). Colors flip without a reload.
3. Edit any text field → the footer status reads `unsaved changes` with an amber dot; the topbar dot also turns amber.
4. Click `Save` → status reads `saved — restart hermind to apply` with a green dot.
5. On a `Secret` field (e.g. any API key), click `Show` → the input unmasks and the button text changes to `Hide`. Click again to re-mask.
6. Resize the browser to ~800px wide — sidebar + main remain usable, no horizontal scroll, inputs remain full width inside the panel.

If any of the six checks fail, stop and report which step broke. Do not patch the CSS blind; re-open the spec section and match it.

- [ ] **Step 4: Shut down the dev server**

Either click `Save & Exit` or `Ctrl-C` the `./bin/hermind config --web` process.

---

## Task 5: Single commit

Per spec §Rollout, the whole refresh ships as one commit.

- [ ] **Step 1: Stage just the three frontend files**

Run: `git add cli/ui/webconfig/web/index.html cli/ui/webconfig/web/app.css cli/ui/webconfig/web/app.js`

- [ ] **Step 2: Verify nothing else is staged**

Run: `git diff --staged --name-only`
Expected exactly:
```
cli/ui/webconfig/web/app.css
cli/ui/webconfig/web/app.js
cli/ui/webconfig/web/index.html
```

- [ ] **Step 3: Commit**

```bash
git commit -m "$(cat <<'EOF'
feat(web-config): refresh UI with light/dark theme and amber accent

Rewrites cli/ui/webconfig/web/{index.html,app.css,app.js} to match the
TUI brand (#FFB800 amber) with a clean Linear/Vercel-style shell:
header bar, bordered panel, amber-strip sidebar active state, colored
status dot, and OS-aware light/dark via prefers-color-scheme. Backend
and all Go code untouched — purely embedded asset refresh.
EOF
)"
```

- [ ] **Step 4: Confirm the commit landed**

Run: `git log -1 --stat`
Expected: 3 files changed under `cli/ui/webconfig/web/`, no other paths.

---

## Rollback

Revert with `git revert <commit>`; no migrations, no config changes, no caches to invalidate. Users get the previous UI on the next build.
