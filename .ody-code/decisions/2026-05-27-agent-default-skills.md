# Decision: All 4 default skills registered in Go v1

**Context:** Node's `DEFAULT_SKILLS` only includes `[memory, docSummarizer, webScraping]`. `rechart` is registered separately when present in workspace agent config.

**Decision:** For Go v1 simplicity, we register **all 4 default skills** (`rag-memory`, `document-summarizer`, `web-scraping`, `rechart`) unconditionally and rely on the global `disabled_agent_skills` SystemSetting to suppress unwanted ones.

**Rationale:** Per-workspace skill config (Node's `workspace.aibitat` approach) requires additional UI + backend complexity. A global disable list is simpler to implement and covers the 90% use case.

**Consequence:** Users upgrading from Node may see `rechart` available when it wasn't before. They can disable it via `disabled_agent_skills: ["rechart"]`.
