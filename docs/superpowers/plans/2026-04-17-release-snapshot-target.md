# `make release-snapshot` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a single `make release-snapshot` Makefile target that cross-compiles hermind for Linux/macOS/Windows × x86/x86_64/arm64 (8 binaries) with archives and checksums, by driving the existing `goreleaser` config.

**Architecture:** Reuse `.goreleaser.yml`. Expand its matrix (add `386`, unskip `windows/arm64`, exclude `darwin/386`). Extend the archive `name_template` to map `386 → x86` (matching the existing `amd64 → x86_64` convention). Add a Makefile target that preflights for the `goreleaser` binary and invokes `goreleaser release --snapshot --skip=publish --clean`. Ignore `dist/` in git.

**Tech Stack:** goreleaser v2, GNU make, Go 1.25 (already on PATH).

---

## File Structure

- Modify: `.goreleaser.yml` — expand `builds.goarch`, update `builds.ignore`, extend `archives.name_template` to handle `386`.
- Modify: `Makefile` — append `release-snapshot` to `.PHONY`, add the new rule.
- Modify: `.gitignore` — add `dist/`.

All changes are additive or conservative (no removals that break existing behavior). Existing `make build`, `make test`, and the release GitHub Action continue to work because the default goreleaser `build` still produces every arch; release.yml calls `goreleaser release` which honors the same config.

---

## Task 1: expand goreleaser build matrix

**Files:**
- Modify: `.goreleaser.yml`

- [ ] **Step 1: Read the current `builds` block**

Run: `sed -n '10,31p' .goreleaser.yml`

Expected: matches the following starting config (verify before editing):

```yaml
builds:
  - id: hermind
    main: ./cmd/hermind
    binary: hermind
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}
      - -X main.Commit={{.ShortCommit}}
      - -X main.BuildDate={{.Date}}
```

If the file doesn't match, stop and report the divergence.

- [ ] **Step 2: Update the `builds` block**

Replace the block above with:

```yaml
builds:
  - id: hermind
    main: ./cmd/hermind
    binary: hermind
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
      - "386"
    ignore:
      - goos: darwin
        goarch: "386"
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}
      - -X main.Commit={{.ShortCommit}}
      - -X main.BuildDate={{.Date}}
```

Changes: added `"386"` to `goarch`; replaced the `windows/arm64` ignore with a `darwin/386` ignore. Note `"386"` must be quoted because YAML would otherwise parse it as integer 386, which goreleaser rejects.

- [ ] **Step 3: Validate goreleaser config**

Run: `goreleaser check`

Expected: `config is valid`. If the command is not found, install: `brew install goreleaser` (macOS) or `go install github.com/goreleaser/goreleaser/v2@latest`.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "build(goreleaser): expand matrix to 8 targets (add 386, windows/arm64)"
```

---

## Task 2: extend archive name template for x86

**Files:**
- Modify: `.goreleaser.yml`

- [ ] **Step 1: Read the current `archives` block**

Run: `sed -n '32,40p' .goreleaser.yml`

Expected:

```yaml
archives:
  - id: hermind
    ids: [hermind]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{- if eq .Arch "amd64" }}x86_64{{ else }}{{ .Arch }}{{ end }}
    files:
      - LICENSE*
      - README*
      - CHANGELOG*
```

- [ ] **Step 2: Update `name_template` to map `386 → x86`**

Replace the `name_template` line with:

```yaml
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_
      {{- if eq .Arch "amd64" }}x86_64
      {{- else if eq .Arch "386" }}x86
      {{- else }}{{ .Arch }}{{ end }}
```

The `>-` folded scalar and `{{-` left-trim directives collapse the template into a single flat string at render time. Result for `386` / linux: `hermind_<version>_linux_x86.tar.gz`.

- [ ] **Step 3: Revalidate**

Run: `goreleaser check`

Expected: `config is valid`.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "build(goreleaser): map 386 to x86 in archive name template"
```

---

## Task 3: add `release-snapshot` Makefile target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read the current Makefile**

Run: `cat Makefile`

Expected first line: `# Makefile`. The `.PHONY` line should currently read:

```makefile
.PHONY: build test lint clean
```

If it doesn't match, stop and report.

- [ ] **Step 2: Extend `.PHONY` and append the new rule**

Edit `Makefile` — change the `.PHONY` line to include the new target:

```makefile
.PHONY: build test lint clean release-snapshot
```

Then append this rule at the bottom of the file (after the existing `clean:` target):

```makefile
release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "error: goreleaser not found. Install with one of:"; \
	  echo "  brew install goreleaser"; \
	  echo "  go install github.com/goreleaser/goreleaser/v2@latest"; \
	  exit 1; \
	}
	goreleaser release --snapshot --skip=publish --clean
```

Note: Makefile rules use tabs, not spaces. Confirm with `cat -A Makefile | tail -10` — the indented lines should start with `^I` (tab), not spaces.

- [ ] **Step 3: Verify Makefile parses**

Run: `make -n release-snapshot`

Expected: prints the commands that would run (the `@command -v ...` guard then `goreleaser release ...`). No errors.

- [ ] **Step 4: Verify preflight error path**

Temporarily rename goreleaser on your PATH and confirm the guard fires:

```bash
# Skip this step if goreleaser is not installed via brew (rename is non-trivial).
which goreleaser
# Example dry-run without goreleaser:
PATH=/usr/bin:/bin make release-snapshot || echo "exit $?"
```

Expected: the three-line error message followed by `exit 1`. Restore your PATH afterwards. If the test is awkward on your setup, skip — the guard is trivial.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "build(make): add release-snapshot target driving goreleaser"
```

---

## Task 4: ignore `dist/` in git

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Inspect current `.gitignore`**

Run: `cat .gitignore`

Expected contents:

```
bin/
*.test
*.out
coverage.txt
.env
```

- [ ] **Step 2: Add `dist/`**

Append a new line. Final contents:

```
bin/
dist/
*.test
*.out
coverage.txt
.env
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore
git commit -m "chore(gitignore): ignore goreleaser dist/ output"
```

---

## Task 5: end-to-end smoke test

No file changes — this is a verification task. If any step fails, return to the task that owns the failure.

- [ ] **Step 1: Clean prior output**

Run: `rm -rf dist/`

- [ ] **Step 2: Full matrix build**

Run: `make release-snapshot`

Expected: goreleaser logs `before hook` → `building` (8 targets) → `archiving` → `computing checksums` → `succeeded after <N>s`. Exit code 0.

- [ ] **Step 3: Verify archive count**

Run: `ls dist/*.tar.gz dist/*.zip`

Expected: 5 `.tar.gz` files (linux/darwin) and 3 `.zip` files (windows) = 8 archives. Names follow `hermind_<version>_<os>_<x86|x86_64|arm64>.{tar.gz,zip}`.

- [ ] **Step 4: Verify checksums file**

Run: `test -f dist/checksums.txt && head -n 3 dist/checksums.txt`

Expected: file exists; lines are `<sha256>  hermind_<version>_<os>_<arch>.<ext>`.

- [ ] **Step 5: Verify arch of one ARM binary**

Run:

```bash
tar -xOzf dist/hermind_*_linux_arm64.tar.gz hermind | file -
```

Expected: output contains `ELF 64-bit LSB ... ARM aarch64`.

- [ ] **Step 6: Verify arch of one 386 binary**

Run:

```bash
tar -xOzf dist/hermind_*_linux_x86.tar.gz hermind | file -
```

Expected: output contains `ELF 32-bit LSB ... Intel 80386`.

- [ ] **Step 7: Cleanup**

Run: `rm -rf dist/`

Nothing to commit for this task.

---

## Self-Review Checklist

1. **Spec coverage:**
   - Expand matrix to 8 binaries ↔ Task 1 ✓
   - Archive name mapping `386 → x86` ↔ Task 2 ✓
   - `make release-snapshot` target with goreleaser preflight ↔ Task 3 ✓
   - Ignore `dist/` in git ↔ Task 4 ✓
   - Smoke test that produces all 8 archives + checksums ↔ Task 5 ✓

2. **Out-of-scope items stay out:** No changes to `install.sh` (deferred per spec). No Docker fallback. No new shell scripts. No FreeBSD/armv7. No publishing changes — `release.yml` workflow untouched.

3. **Placeholder scan:** No TBDs, no "add error handling", no "similar to task N". Each step has exact commands and expected output.

4. **Type consistency:** N/A — this is a build-config plan with no Go types. YAML keys (`goarch`, `goos`, `ignore`) used identically across tasks.

5. **Gaps / risks:**
   - If `windows/arm64` cross-compile fails (unlikely — Go 1.25 supports it well), Task 1 will surface it during `goreleaser check`; no code change needed, just investigation.
   - If goreleaser isn't installed, Task 1 Step 3 fails early with the install hint. Install it and retry.

---

## Definition of Done

- `goreleaser check` reports `config is valid`.
- `make release-snapshot` produces 8 archives + `checksums.txt` + raw binary trees under `dist/`.
- `file` reports the correct architecture for the 386 and arm64 binaries.
- `git status` shows no untracked `dist/` entries (gitignore works).
- `make build`, `make test`, and `.github/workflows/release.yml` remain green — none were modified, but confirm with a quick `make build` and the release workflow's next dry-run.
