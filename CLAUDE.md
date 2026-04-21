## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill
tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.
The skill has specialized workflows that produce better results than ad-hoc answers.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review
- Save progress, checkpoint, resume → invoke checkpoint
- Code quality, health check → invoke health

## Design System

Always read `DESIGN.md` before making any visual or UI change. All font
choices, colors, spacing, aesthetic direction, and component patterns live
there. Do not deviate without explicit user approval.

Specifically:
- Never use `system-ui`, `Inter`, `Roboto`, `Arial`, `Helvetica`, or any
  blacklisted font.
- Stick to the amber `#FFB800` accent — no purple, no gradient CTAs.
- Body text is 13px (not 16px). Mono headings (not sans). 4px spacing base.
- 1px borders, 2–4px radii, minimal motion. Anti-patterns list in DESIGN.md.

In QA / review mode, flag any code that drifts from DESIGN.md.
