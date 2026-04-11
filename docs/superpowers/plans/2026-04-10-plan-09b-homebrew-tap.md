# Plan 9b: Homebrew Tap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Let macOS and Linux users install hermes with `brew install nousresearch/tap/hermes` by publishing a Homebrew formula to a dedicated tap repo as part of the goreleaser release pipeline.

**Architecture:** Goreleaser has first-class Homebrew support via `brews:` in `.goreleaser.yml`. On tag push, after the binaries and archives are built, goreleaser generates a formula file and commits it to a separate tap repository (default: `homebrew-tap` under the same GitHub org). Users add the tap with `brew tap nousresearch/tap` (one-time) and then `brew install hermes`.

**Tech Stack:** goreleaser + an existing GitHub PAT (`HOMEBREW_TAP_TOKEN`) stored as a repo secret. No Go code changes.

**Deliverable at end of plan:**
```
$ brew tap nousresearch/tap
$ brew install hermes
$ hermes version
hermes-agent v0.1.0
  commit:     abc1234
  built:      2026-04-11T12:00:00Z
  go:         go1.25.7
```

**Non-goals (deferred):**
- Signed bottles — later
- Auto-upgrade prompts in the CLI — later
- `hermes upgrade` subcommand — later

---

## Task 1: Extend .goreleaser.yml with a brews block

- [ ] Add to `.goreleaser.yml` (right after `archives:`):

```yaml
brews:
  - name: hermes
    ids: [hermes]
    homepage: https://github.com/nousresearch/hermes-agent-go
    description: "Hermes Agent — Go port of the hermes AI agent framework"
    license: MIT
    repository:
      owner: nousresearch
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    commit_author:
      name: hermes-release-bot
      email: hermes-release-bot@users.noreply.github.com
    commit_msg_template: "brew: update hermes to {{ .Tag }}"
    install: |
      bin.install "hermes"
    test: |
      system "#{bin}/hermes", "version"
```

- [ ] Verify the template loads in snapshot mode: `goreleaser check` (no token needed).
- [ ] Commit `feat(release): publish Homebrew formula via goreleaser brews block`.

---

## Task 2: Document the install flow

- [ ] Update `scripts/install.sh` header comment to mention Homebrew as an alternative.
- [ ] Add a short "Install" section to the existing Plan 9 deliverable note in `docs/superpowers/plans/2026-04-10-plan-09-release.md` (append, don't rewrite).
- [ ] Commit `docs(release): document Homebrew install path`.

---

## Task 3: Release workflow secret

- [ ] No code change — this is a one-time repo setting. The release.yml workflow already passes `GITHUB_TOKEN`; we just need `HOMEBREW_TAP_TOKEN` as a repo secret for cross-repo write. Add a README note under `hermes-agent-go/.github/workflows/release.yml` via a comment block at the top explaining the requirement.
- [ ] Commit `ci: document HOMEBREW_TAP_TOKEN secret requirement`.

---

## Verification Checklist

- [ ] `goreleaser check` (or the equivalent) parses `.goreleaser.yml` without errors
- [ ] The release workflow's `env` block includes `HOMEBREW_TAP_TOKEN`
- [ ] Documentation mentions `brew tap nousresearch/tap && brew install hermes`
