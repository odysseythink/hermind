# TODOs

## Design

- [ ] **Create DESIGN.md via /design-consultation**
  - **Why:** The plan has inline design tokens (symbols, colors, spacing) but no canonical design system document. Without DESIGN.md, each contributor makes independent visual decisions.
  - **Pros:** Single source of truth for all CLI visual decisions. Enables consistency across skins. Makes onboarding new contributors easier.
  - **Cons:** Takes ~30 min to run /design-consultation. May need iteration.
  - **Context:** Design tokens are currently embedded in the spec file (Section 7). They should be extracted into a standalone DESIGN.md that covers the full CLI design system.
  - **Depends on:** None. Can be done before or during implementation.
