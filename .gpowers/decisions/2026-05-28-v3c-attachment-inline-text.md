# v3-C — Attachment Inline-Text Strategy

**Date**: 2026-05-28  
**Status**: Adopted

Gmail/Outlook v1 attachments follow a simplified "parse to text + inline into mail body" path. **Binary attachments are not sent.** Rationale: both Apps Script and Graph API binary attachment schemas are complex; the Node side already does the same; 90% of use cases are about letting the LLM see attachment content.
