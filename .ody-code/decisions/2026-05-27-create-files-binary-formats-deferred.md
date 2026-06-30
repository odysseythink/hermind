# Create-Files Skill — Binary Formats Deferred to PR-AR-6.1

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Node's create-files-agent supports txt/md/docx/pdf/pptx/xlsx. Binary formats need unioffice or similar — non-trivial new dep + license discussion.

**Decision**: PR-AR-6 ships txt/md only. docx/pdf/pptx/xlsx return `tool.Error("format X not yet implemented; PR-AR-6.1")`.

**Rationale**: txt/md is 90% of agent file-generation use cases (memos, reports). The four binary formats are a separate concern that justifies its own license + dependency review.
