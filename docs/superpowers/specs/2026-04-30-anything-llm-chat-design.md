# Design: Port anything-llm Chat UI to hermind

## Background

hermind's current web chat interface (`ChatWorkspace`, `MessageList`, `MessageBubble`, `ComposerBar`) is minimal and terminal-styled. Many users interact with hermind directly through the web UI without configuring IM gateways. The anything-llm project's chat interface provides a significantly better user experience with rounded message bubbles, smart scrolling, message actions (copy/edit/delete/regenerate), attachments, TTS, speech input, and a Sources sidebar.

## Goals

- Replace hermind's chat area with a full anything-llm-style chat experience.
- Synchronously extend the backend APIs and storage layer to support all new frontend features.
- Keep hermind's existing app shell (TopBar, Settings panel, hash-based routing) untouched.
- Use CSS Modules for all styling (no Tailwind CSS).

## Non-Goals

- Do not change the overall application layout to anything-llm's multi-page shell.
- Do not replace React Router (hermind has none; keep hash-based mode switching).
- Do not introduce new frontend dependencies beyond what is strictly necessary.

## Architecture Overview

The replacement is scoped to the `ChatWorkspace` component tree within `web/src/App.tsx`. The existing app shell (TopBar, SettingsSidebar, SettingsPanel, Footer) remains unchanged.

Key principles:
- **State layer**: Extend the existing `chatReducer` + `useReducer` architecture with new action types.
- **Data flow**: Keep SSE (`/api/sse`) for streaming; add conventional REST endpoints for mutations.
- **Component communication**: Replace anything-llm's `CustomEvent` pattern with React props/callbacks.
- **Styling**: Rewrite all component styles with CSS Modules, mapping anything-llm's Tailwind classes to hermind's design tokens where possible.

## Data Model

### Extended ChatMessage

```ts
type ChatMessage = {
  id: string;                  // frontend temporary ID
  role: string;
  content: string;
  timestamp: number;

  // new fields
  chatId?: number;             // backend persistent message ID
  attachments?: Attachment[];
  sources?: Source[];
  feedbackScore?: number | null; // 1 = thumbs up, -1 = thumbs down, null = none
  metrics?: MessageMetrics;
  outputs?: Output[];
  error?: boolean;
  pending?: boolean;           // awaiting LLM response
  animate?: boolean;           // currently streaming
};

type Attachment = {
  id: string;
  name: string;
  type: string;
  url: string;
  size: number;
};

type Source = {
  id: string;
  title: string;
  text: string;
  metadata?: Record<string, unknown>;
};

type MessageMetrics = {
  promptTokens?: number;
  completionTokens?: number;
  latencyMs?: number;
};
```

### Extended ChatState

```ts
type ChatState = {
  messages: ChatMessage[];
  composer: {
    text: string;
    attachments: Attachment[]; // new
  };
  streaming: StreamingState;
  suggestions: string[];       // new: empty-state suggestions
};
```

## Backend API Extensions

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/conversation/messages/:id` | PUT | Edit a message's content |
| `/api/conversation/messages/:id` | DELETE | Delete a message and all messages after it |
| `/api/conversation/messages/:id/regenerate` | POST | Regenerate the assistant reply |
| `/api/upload` | POST | Upload an attachment (multipart/form-data) |
| `/api/feedback` | POST | Submit thumbs up/down feedback for a message |
| `/api/suggestions` | GET | Get suggested starter messages |
| `/api/tts` | POST | Synthesize text to speech |

## Storage Interface Extensions

```go
type Storage interface {
    // existing methods...

    // new methods
    UpdateMessage(ctx context.Context, id int64, content string) error
    DeleteMessage(ctx context.Context, id int64) error
    DeleteMessagesAfter(ctx context.Context, id int64) error
    SaveFeedback(ctx context.Context, messageID int64, score int) error

    // attachments (stored on local filesystem or as SQLite blobs)
    SaveAttachment(ctx context.Context, msgID int64, name string, data []byte) (url string, err error)
    ListAttachments(ctx context.Context, msgID int64) ([]Attachment, error)
}
```

## Component Structure

```
ChatWorkspace (root, keeps existing props)
├── EmptyState (new)                     // welcome screen when no messages
│   ├── Greeting
│   ├── SuggestedMessages
│   └── QuickActions
├── ChatHistory (replaces MessageList)
│   ├── HistoricalMessage (replaces MessageBubble)
│   │   ├── Avatar                       // user vs assistant avatar
│   │   ├── RenderChatContent            // markdown + attachments + code blocks
│   │   ├── Actions                      // copy / edit / delete / regenerate / feedback
│   │   ├── Citations                    // inline source references
│   │   └── ChatAttachments              // attachment previews
│   ├── PromptReply (new)                // streaming assistant bubble
│   ├── StatusResponse (new)             // agent status / tool-call notices
│   └── ScrollToBottomButton (new)       // appears when not at bottom
├── PromptInput (replaces ComposerBar)
│   ├── TextArea (auto-growing)          // replaces fixed-rows textarea
│   ├── Attachments (new)                // uploaded file previews
│   ├── AttachmentUploader (new)         // drag-and-drop + click upload
│   ├── ToolsMenu (new)                  // @mentions / slash commands
│   └── StopGenerationButton (kept)
└── SourcesSidebar (new)                 // collapsible source panel
```

## Styling Decisions (CSS Modules)

- User message bubble: `background: var(--surface-2); border-radius: 20px 20px 4px 20px;`
- Assistant message: left-aligned, with avatar, no background bubble
- Input bar: fixed bottom, large rounded border, auto-growing via `element.scrollHeight`
- Scrollbar: custom `::-webkit-scrollbar` matching the dark theme
- No Tailwind utility classes; all styles in `.module.css` files

## Implementation Phases

### Phase 1 — Core Skeleton (Minimum Viable Experience)

Frontend:
- `EmptyState` welcome page with greeting and quick actions
- `ChatHistory` with smart scrolling (bottom detection, user-scroll awareness, scroll-to-bottom button)
- `HistoricalMessage` styled as rounded bubbles with avatars and role indicators
- `PromptInput` with auto-growing textarea, Enter-to-send / Shift+Enter newline
- `Actions` message operation menu (copy, edit, delete, regenerate)
- `PromptReply` streaming reply bubble

Backend:
- `Storage.UpdateMessage` + `PUT /api/conversation/messages/:id`
- `Storage.DeleteMessage` / `DeleteMessagesAfter` + `DELETE /api/conversation/messages/:id`
- `POST /api/conversation/messages/:id/regenerate`
- Extend `StoredMessage` schema with `chat_id`

### Phase 2 — Rich Media & Interaction

Frontend:
- `AttachmentUploader` drag-and-drop + click upload
- `Attachments` preview / remove list
- `SourcesSidebar` source reference panel
- `Citations` inline source rendering
- TTS button + audio playback
- Speech-to-Text voice input

Backend:
- `POST /api/upload` + file storage
- `POST /api/tts` + TTS provider integration
- `GET /api/suggestions`

### Phase 3 — Advanced Features

Frontend:
- `ThoughtContainer` think-chain collapse/expand
- `Chartable` chart visualization
- Message feedback (thumbs up/down)
- Message metrics display (token count, latency)

Backend:
- `POST /api/feedback`
- Engine outputs thinking/reasoning content over SSE
- Chart data endpoint

## Error Handling

| Scenario | Strategy |
|----------|----------|
| Edit message fails | Toast error, rollback local edit, preserve original message |
| Delete message fails | Toast error, do not remove from UI (optimistic with rollback) |
| Regenerate fails | Restore original assistant message, reset stream state to idle |
| Attachment upload fails | Show red error state on attachment item, allow retry or remove |
| SSE disconnect/error | Keep existing mechanism: error banner, streaming state → error |
| Backend 404 (message gone) | Refresh history to resync, Toast "Message expired" |

**Optimistic updates**: Edit, delete, and feedback use optimistic UI updates (modify UI first, then call API), with rollback on failure. Send and regenerate keep the existing "call API first, then update UI" pattern.

## Testing Strategy

### Go Backend
- `storage/sqlite/`: unit tests for `UpdateMessage`, `DeleteMessage`, `DeleteMessagesAfter`
- `api/`: handler tests for new endpoints using `httptest` + in-memory SQLite
- `agent/`: regenerate logic (either in engine layer or in new API layer)

### TypeScript Frontend
- `state/chat.ts`: reducer tests for each new action type
- Component tests: `ChatHistory` scroll behavior, `HistoricalMessage` action menu interactions, `PromptInput` auto-growing
- Follow existing `*.test.tsx` style (`@testing-library/react` + vitest)

### Regression Gates
- After each phase: `make web-check` (type-check + test + lint + build)
- After each phase: `go test -race -cover ./...`

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Large code volume increases bug surface | Three-phase delivery; each phase is independently testable and deployable |
| CSS Modules rewrite diverges from anything-llm visual fidelity | Reference screenshots from anything-llm for visual QA |
| Backend storage migrations needed | Add new migrations in `storage/sqlite/migrate.go`; never modify existing migration SQL |
| SSE event types may need extension for new features | Extend `wireEngineToHub` to publish new event types; keep backward-compatible |
