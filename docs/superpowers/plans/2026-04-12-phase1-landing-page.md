# Phase 1: Landing Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port and adapt the Python reference landing page into `website/` as a standalone marketing site for the Go rewrite of Hermes Agent.

**Architecture:** Static HTML/CSS/JS — no build step, no Node.js. Three files (`index.html`, `style.css`, `script.js`) plus image assets. Content updated to reflect the Go rewrite's capabilities. Deployable to GitHub Pages.

**Tech Stack:** HTML5, CSS3 (custom properties, grid, responsive), vanilla JS (IntersectionObserver, clipboard API, Three.js r128 via CDN)

---

## File Structure

```
website/
├── index.html              # Main page — hero, install, demo, features, specs, footer
├── style.css               # Full styling with Nous Blue palette, responsive breakpoints
├── script.js               # Platform detection, copy-to-clipboard, terminal demo, noise overlay
├── nous-logo.png           # Copied from reference
├── hermes-agent-banner.png # Copied from reference
├── apple-touch-icon.png    # Copied from reference
├── icon-512.png            # Copied from reference
├── icon-192.png            # Copied from reference
├── favicon.ico             # Copied from reference
├── favicon-32x32.png       # Copied from reference
└── favicon-16x16.png       # Copied from reference
```

All files live at the top level of `website/` — no subdirectories. The reference landing page at `hermes-agent-2026.4.8/landingpage/` has the same flat structure.

---

### Task 1: Copy image assets

**Files:**
- Copy: `hermes-agent-2026.4.8/landingpage/*.png` → `website/`
- Copy: `hermes-agent-2026.4.8/landingpage/favicon.ico` → `website/`

- [ ] **Step 1: Create website directory and copy all image/favicon assets**

```bash
mkdir -p website
cp hermes-agent-2026.4.8/landingpage/nous-logo.png website/
cp hermes-agent-2026.4.8/landingpage/hermes-agent-banner.png website/
cp hermes-agent-2026.4.8/landingpage/apple-touch-icon.png website/
cp hermes-agent-2026.4.8/landingpage/icon-512.png website/
cp hermes-agent-2026.4.8/landingpage/icon-192.png website/
cp hermes-agent-2026.4.8/landingpage/favicon.ico website/
cp hermes-agent-2026.4.8/landingpage/favicon-32x32.png website/
cp hermes-agent-2026.4.8/landingpage/favicon-16x16.png website/
```

- [ ] **Step 2: Verify all assets copied**

```bash
ls -la website/
```

Expected: 8 image/icon files, all with non-zero sizes.

- [ ] **Step 3: Commit**

```bash
git add website/
git commit -m "feat(website): copy image assets from reference landing page"
```

---

### Task 2: Create style.css

**Files:**
- Create: `website/style.css`

The CSS is copied from `hermes-agent-2026.4.8/landingpage/style.css` **with no changes**. The entire 1,179-line stylesheet works as-is — it uses CSS custom properties, has no references to Python-specific content, and all class names match the HTML we'll create in Task 3.

- [ ] **Step 1: Copy style.css from reference**

```bash
cp hermes-agent-2026.4.8/landingpage/style.css website/style.css
```

- [ ] **Step 2: Verify the file**

```bash
wc -l website/style.css
```

Expected: 1179 lines.

- [ ] **Step 3: Commit**

```bash
git add website/style.css
git commit -m "feat(website): add landing page stylesheet"
```

---

### Task 3: Create index.html with updated content

**Files:**
- Create: `website/index.html`

Copy `hermes-agent-2026.4.8/landingpage/index.html` and apply the following content changes to reflect the Go rewrite. Structural HTML stays identical — only text content and URLs change.

- [ ] **Step 1: Copy index.html from reference**

```bash
cp hermes-agent-2026.4.8/landingpage/index.html website/index.html
```

- [ ] **Step 2: Update install command for Go binary**

In `website/index.html`, replace all occurrences of the Python install command with the Go binary install command. There are 4 places this appears:

Find (in the hero install widget `<code id="install-command">`):
```html
curl -fsSL
                https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh
                | bash
```

Replace with:
```html
curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash
```

Find (in `data-text` attribute of `id="step1-copy"` button):
```
data-text="curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash"
```

Replace with:
```
data-text="curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash"
```

Find (in the step 1 `<code id="step1-command">`):
```html
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Replace with:
```html
curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash
```

- [ ] **Step 3: Update step 1 install note**

Find:
```html
<p class="step-note" id="step1-note">
                Installs uv, Python 3.11, clones the repo, sets up everything.
                No sudo needed.
              </p>
```

Replace with:
```html
<p class="step-note" id="step1-note">
                Downloads a single Go binary. No Python, no dependencies, no sudo needed.
              </p>
```

- [ ] **Step 4: Update step 2 configure note**

Find:
```html
<p class="step-note">
                Connect to Nous Portal (OAuth), OpenRouter (API key), or your
                own endpoint.
              </p>
```

Replace with:
```html
<p class="step-note">
                Connect to Nous Portal (OAuth), OpenRouter, or any of 8+
                supported LLM providers.
              </p>
```

- [ ] **Step 5: Update step 4 platforms note**

Find:
```html
<p class="step-note">
                Walk through connecting Telegram, Discord, Slack, or WhatsApp.
                Runs as a systemd service.
              </p>
```

Replace with:
```html
<p class="step-note">
                Walk through connecting any of 21 platforms — Telegram, Discord,
                Slack, WhatsApp, Signal, Matrix, and more. Runs as a systemd service.
              </p>
```

- [ ] **Step 6: Update step 5 update command**

Find:
```html
<pre><code>hermes update</code></pre>
```

Replace with:
```html
<pre><code>hermes upgrade</code></pre>
```

Also update the corresponding copy button `data-text`:

Find:
```
data-text="hermes update"
```

Replace with:
```
data-text="hermes upgrade"
```

- [ ] **Step 7: Update Windows note**

Find:
```html
<p>
            Native Windows support is extremely experimental and unsupported.
            Please install
            <a
              href="https://learn.microsoft.com/en-us/windows/wsl/install"
              target="_blank"
              rel="noopener"
              >WSL2</a
            >
            and run Hermes Agent from there.
          </p>
```

Replace with:
```html
<p>
            Native Windows support is coming soon. For now, please install
            <a
              href="https://learn.microsoft.com/en-us/windows/wsl/install"
              target="_blank"
              rel="noopener"
              >WSL2</a
            >
            and run Hermes Agent from there.
          </p>
```

- [ ] **Step 8: Update features — "Lives Where You Do" card**

Find:
```html
<p>
              Telegram, Discord, Slack, WhatsApp, and CLI from a single gateway
              — start on one, pick up on another.
            </p>
```

Replace with:
```html
<p>
              21 platforms — Telegram, Discord, Slack, WhatsApp, Signal, Matrix,
              Email, and more — from a single gateway. Start on one, pick up on
              another.
            </p>
```

- [ ] **Step 9: Update features — "Real Sandboxing" card**

Find:
```html
<p>
              Five backends — local, Docker, SSH, Singularity, Modal — with
              container hardening and namespace isolation.
            </p>
```

Replace with:
```html
<p>
              Six backends — local, Docker, SSH, Singularity, Modal, Daytona —
              with container hardening and namespace isolation.
            </p>
```

- [ ] **Step 10: Update specs — Tools row**

Find:
```html
<p class="spec-value">
                40+ built-in — web search, terminal, file system, browser
                automation, vision, image generation, text-to-speech, code
                execution, subagent delegation, memory, task planning, cron
                scheduling, multi-model reasoning, and more.
              </p>
```

Replace with:
```html
<p class="spec-value">
                15+ built-in — web search, terminal, file system, browser
                automation, vision, image generation, text-to-speech,
                transcription, memory, security checks, MCP client, task
                planning, cron scheduling, and more.
              </p>
```

- [ ] **Step 11: Update specs — Platforms row**

Find:
```html
<p class="spec-value">
                Telegram, Discord, Slack, WhatsApp, Signal, Email, and CLI — all
                from a single gateway. Connect to
                <a
                  href="https://portal.nousresearch.com"
                  target="_blank"
                  rel="noopener"
                  >Nous Portal</a
                >, OpenRouter, or any OpenAI-compatible API.
              </p>
```

Replace with:
```html
<p class="spec-value">
                21 platforms — Telegram, Discord, Slack, WhatsApp, Signal,
                Matrix, Email, SMS, HomeAssistant, WeChat, DingTalk, Feishu, and
                more — all from a single gateway. Connect to
                <a
                  href="https://portal.nousresearch.com"
                  target="_blank"
                  rel="noopener"
                  >Nous Portal</a
                >, OpenRouter, or any of 8+ LLM providers.
              </p>
```

- [ ] **Step 12: Update specs — Environments row**

Find:
```html
<p class="spec-value">
                Run locally, in Docker, over SSH, on Modal, Daytona, or
                Singularity. Container hardening with read-only root, dropped
                capabilities, and namespace isolation.
              </p>
```

Replace with:
```html
<p class="spec-value">
                Run locally, in Docker, over SSH, on Modal, Daytona, or
                Singularity. Single static Go binary — no Python, no
                dependencies. Container hardening with namespace isolation.
              </p>
```

- [ ] **Step 13: Update specs — Skills row**

Find:
```html
<p class="spec-value">
                40+ bundled skills covering MLOps, GitHub workflows, research,
                and more. The agent creates new skills on the fly and shares
                them via the open
                <a href="https://agentskills.io" target="_blank" rel="noopener"
                  >agentskills.io</a
                >
                format. Install community skills from
                <a href="https://clawhub.ai" target="_blank" rel="noopener"
                  >ClawHub</a
                >,
                <a href="https://lobehub.com" target="_blank" rel="noopener"
                  >LobeHub</a
                >, and GitHub.
              </p>
```

Replace with:
```html
<p class="spec-value">
                Extensible skill system with dynamic tool injection and slash
                commands. The agent creates new skills on the fly and shares
                them via the open
                <a href="https://agentskills.io" target="_blank" rel="noopener"
                  >agentskills.io</a
                >
                format. Install community skills from
                <a href="https://clawhub.ai" target="_blank" rel="noopener"
                  >ClawHub</a
                >,
                <a href="https://lobehub.com" target="_blank" rel="noopener"
                  >LobeHub</a
                >, and GitHub.
              </p>
```

- [ ] **Step 14: Update hero subtitle**

Find:
```html
<p class="hero-subtitle">
          It's not a coding copilot tethered to an IDE or a chatbot wrapper
          around a single API. It's an <strong>autonomous agent</strong> that
          lives on your server, remembers what it learns, and gets more capable
          the longer it runs.
        </p>
```

Replace with:
```html
<p class="hero-subtitle">
          A single Go binary — no Python, no dependencies. It's an
          <strong>autonomous agent</strong> that lives on your server, remembers
          what it learns, and gets more capable the longer it runs.
        </p>
```

- [ ] **Step 15: Open in browser and visually verify**

```bash
open website/index.html
```

Verify:
- Page loads with Nous Blue color scheme
- ASCII art banner displays correctly
- Install command shows the Go binary URL
- All 5 install steps render with updated copy
- Feature cards show updated platform/backend counts
- Specs section expands with updated content
- Mobile hamburger menu works (resize browser)

- [ ] **Step 16: Commit**

```bash
git add website/index.html
git commit -m "feat(website): add landing page HTML with Go rewrite content"
```

---

### Task 4: Create script.js with updated install commands

**Files:**
- Create: `website/script.js`

Copy `hermes-agent-2026.4.8/landingpage/script.js` and update the `PLATFORMS` config object to match the new Go binary install command.

- [ ] **Step 1: Copy script.js from reference**

```bash
cp hermes-agent-2026.4.8/landingpage/script.js website/script.js
```

- [ ] **Step 2: Update the PLATFORMS config object**

Find (lines 6-14):
```javascript
const PLATFORMS = {
  linux: {
    command:
      "curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash",
    prompt: "$",
    note: "Works on Linux, macOS & WSL2 · No prerequisites · Installs everything automatically",
    stepNote:
      "Installs uv, Python 3.11, clones the repo, sets up everything. No sudo needed.",
  },
};
```

Replace with:
```javascript
const PLATFORMS = {
  linux: {
    command:
      "curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash",
    prompt: "$",
    note: "Works on Linux, macOS & WSL2 · No prerequisites · Single Go binary",
    stepNote:
      "Downloads a single Go binary. No Python, no dependencies, no sudo needed.",
  },
};
```

- [ ] **Step 3: Update terminal demo — change Python test command to Go test**

Find (lines 253-256):
```javascript
    {
      type: "output",
      lines: [
        '<span class="t-dim">  python -m pytest tests/ -x                           3.2s</span>',
```

Replace with:
```javascript
    {
      type: "output",
      lines: [
        '<span class="t-dim">  go test ./...                                        3.2s</span>',
```

- [ ] **Step 4: Verify script.js works**

Open `website/index.html` in a browser. Verify:
- Copy button copies the updated Go binary URL
- Terminal demo plays through all 3 scenarios
- Terminal demo shows `go test` instead of `python -m pytest`
- Noise overlay renders (Three.js loads from CDN)
- Scroll animations trigger on feature cards and install steps

- [ ] **Step 5: Commit**

```bash
git add website/script.js
git commit -m "feat(website): add landing page JavaScript with Go rewrite content"
```

---

### Task 5: Add Homebrew install tab

**Files:**
- Modify: `website/index.html`
- Modify: `website/script.js`

The Go rewrite uses goreleaser which generates a Homebrew tap. Add a macOS/Homebrew tab alongside the existing curl tab.

- [ ] **Step 1: Update PLATFORMS in script.js to add brew option**

Find:
```javascript
const PLATFORMS = {
  linux: {
    command:
      "curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash",
    prompt: "$",
    note: "Works on Linux, macOS & WSL2 · No prerequisites · Single Go binary",
    stepNote:
      "Downloads a single Go binary. No Python, no dependencies, no sudo needed.",
  },
};
```

Replace with:
```javascript
const PLATFORMS = {
  linux: {
    command:
      "curl -fsSL https://github.com/NousResearch/hermes-agent/releases/latest/download/install.sh | bash",
    prompt: "$",
    note: "Works on Linux, macOS & WSL2 · No prerequisites · Single Go binary",
    stepNote:
      "Downloads a single Go binary. No Python, no dependencies, no sudo needed.",
  },
  brew: {
    command: "brew install NousResearch/tap/hermes-agent",
    prompt: "$",
    note: "macOS & Linux · Requires Homebrew · Auto-updates with brew upgrade",
    stepNote:
      "Installs via Homebrew tap. Auto-updates with brew upgrade.",
  },
};
```

- [ ] **Step 2: Update detectPlatform to detect macOS**

Find:
```javascript
function detectPlatform() {
  return "linux";
}
```

Replace with:
```javascript
function detectPlatform() {
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes("mac")) return "brew";
  return "linux";
}
```

- [ ] **Step 3: Add Homebrew tab button in hero install widget**

In `website/index.html`, find the install tabs in the hero section:
```html
              <div class="install-tabs">
                <button
                  class="install-tab active"
                  data-platform="linux"
                  onclick="switchPlatform('linux')"
                >
                  Linux / macOS / WSL
                </button>
              </div>
```

Replace with:
```html
              <div class="install-tabs">
                <button
                  class="install-tab active"
                  data-platform="linux"
                  onclick="switchPlatform('linux')"
                >
                  curl (Linux / macOS / WSL)
                </button>
                <button
                  class="install-tab"
                  data-platform="brew"
                  onclick="switchPlatform('brew')"
                >
                  Homebrew
                </button>
              </div>
```

- [ ] **Step 4: Add Homebrew tab button in step 1 code block**

Find the step 1 code tabs:
```html
                  <div class="code-tabs">
                    <button
                      class="code-tab active"
                      data-platform="linux"
                      onclick="switchStepPlatform('linux')"
                    >
                      Linux / macOS / WSL
                    </button>
                  </div>
```

Replace with:
```html
                  <div class="code-tabs">
                    <button
                      class="code-tab active"
                      data-platform="linux"
                      onclick="switchStepPlatform('linux')"
                    >
                      curl (Linux / macOS / WSL)
                    </button>
                    <button
                      class="code-tab"
                      data-platform="brew"
                      onclick="switchStepPlatform('brew')"
                    >
                      Homebrew
                    </button>
                  </div>
```

- [ ] **Step 5: Verify tab switching works**

Open `website/index.html` in a browser. Verify:
- On macOS: Homebrew tab is auto-selected, shows `brew install NousResearch/tap/hermes-agent`
- Clicking "curl" tab switches to the curl command in both hero and step 1
- Clicking "Homebrew" tab switches back
- Copy button copies the correct command for the active tab
- Step 1 note updates to match the selected platform

- [ ] **Step 6: Commit**

```bash
git add website/index.html website/script.js
git commit -m "feat(website): add Homebrew install tab with platform detection"
```

---

### Task 6: Final verification and cleanup

**Files:**
- Review: all files in `website/`

- [ ] **Step 1: Full visual test in browser**

```bash
open website/index.html
```

Walk through the entire page:
1. Hero section: badge, ASCII art, title, subtitle, install widget with 2 tabs
2. Install steps: 5 steps with updated content, tab switching works
3. Terminal demo: all 3 scenarios play, `go test` shows instead of `python -m pytest`
4. Features grid: 6 cards with updated counts (21 platforms, 6 backends)
5. Specs accordion: expands/collapses, content reflects Go rewrite
6. Footer: links work
7. Mobile: resize to <640px, hamburger menu works, single-column layout

- [ ] **Step 2: Check no Python references remain**

```bash
grep -ri "python" website/
grep -ri "uv," website/
grep -ri "pip" website/
```

Expected: No matches (Python references have been replaced). If any remain, fix them.

- [ ] **Step 3: Verify all internal links**

Check that these links in `website/index.html` point to valid targets:
- `#install` → section exists
- `#features` → section exists
- `/docs/` → placeholder (expected, documented in spec)
- GitHub link → `https://github.com/NousResearch/hermes-agent`
- Discord link → `https://discord.gg/NousResearch`

- [ ] **Step 4: Commit any final fixes**

If any issues found in steps 1-3, fix and commit:

```bash
git add website/
git commit -m "fix(website): final cleanup and verification"
```
