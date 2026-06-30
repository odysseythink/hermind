# Apps Script — Go Project Owns Its Own Template

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Node ships an Apps Script template; admins deploy and store deploymentId+apiKey in SystemSetting. Go could reuse the Node-deployed script.

**Decision**: Go ships its own template under `assets/apps-script/{gmail,gcal}/`. Admins running both Node and Go can either deploy twice (separate scripts) or share — protocol is wire-compatible (same action names, same envelope). Go ownership lets us tighten the protocol in future without coordinating with Node releases.
