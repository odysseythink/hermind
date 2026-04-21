# Feishu Long-Connection (Self-Built App) — Design Spec

**Date:** 2026-04-21
**Status:** Draft — awaiting user review
**Scope:** Replace the existing outbound-only Feishu bot (custom-bot webhook) with a bidirectional adapter driven by a Feishu Open Platform self-built app over a long-connection (WebSocket) event channel.

---

## 1. Goals

- Swap the `feishu` gateway adapter from one-way bot-webhook to bidirectional: receive chat events over Feishu's long-connection and send replies via Open API using `app_id` / `app_secret` credentials.
- Reuse the existing `gateway.Platform` interface and descriptor framework. The registry, config loader, and gateway routing stay unchanged.
- Support both Feishu (feishu.cn) and Lark (larksuite.com) deployments via a single `domain` enum field.
- Support both reply-to-source (when an inbound event triggered the reply) and pure push (via optional `default_chat_id`).

## 2. Non-goals

- Rich message types. First version handles only `msg_type == "text"`. Image / file / card / sticker events are dropped at ingest.
- Card actions, reaction events, read receipts — not wired.
- Multiple Feishu tenants in a single adapter instance. Multi-tenancy happens at the framework level (`config.Gateway.Platforms` is a map, users can register multiple named instances each with its own credentials).
- Preserving the old `webhook_url` field as a fallback. This is a hard breaking change with a migration error surfaced at startup.

## 3. Architecture overview

```
┌──────────────────────────────────────────────────────────┐
│  cli/gateway.go buildPlatform                            │
│    ↓ (descriptor.Build(opts))                            │
│  platforms.Descriptor{Type:"feishu"}.Build               │
│    ↓                                                      │
│  platforms.NewFeishuApp(opts) → *FeishuApp               │
└──────────────────────────────────────────────────────────┘
                    │
                    ↓
┌──────────────────────────────────────────────────────────┐
│  FeishuApp                                                │
│    stream  feishuEventStream   // interface, WS events   │
│    sender  feishuMessageSender // interface, REST send   │
│    defaultChatID string                                   │
│                                                           │
│    Run(ctx, handler):                                     │
│      return stream.Start(ctx) // blocks until ctx.Done   │
│      (event callback bound at construction)              │
│                                                           │
│    SendReply(ctx, out):                                   │
│      chat := out.ChatID or defaultChatID                 │
│      sender.Create(ctx, chat, out.Text)                  │
└──────────────────────────────────────────────────────────┘
```

Production wiring:

- `stream` = thin wrapper around `github.com/larksuite/oapi-sdk-go/v3` websocket client (`larkws`) with an `im.message.receive_v1` handler registered at construction.
- `sender` = thin wrapper around `lark.Client.Im.Message.Create`.

## 4. Package layout

- `gateway/platforms/feishu_app.go` (new) — `FeishuApp` struct, `Run`, `SendReply`, interface definitions, SDK-backed production implementations, event decoding + @-mention stripping.
- `gateway/platforms/feishu_app_test.go` (new) — unit tests using fake `feishuEventStream` / `feishuMessageSender`.
- `gateway/platforms/descriptor_feishu.go` (existing, rewritten) — replaces the single-field webhook descriptor with the 5-field self-built-app descriptor; `Build` calls `NewFeishuApp`.
- `gateway/platforms/chatbots.go` — delete `NewFeishu(url)` (lines 29–38).
- `gateway/platforms/chatbots_test.go` — delete the `feishu` subtest case (lines 56–68).
- `go.mod` / `go.sum` — add `github.com/larksuite/oapi-sdk-go/v3` dependency.
- `docs/smoke/feishu-app.md` (new) — manual smoke flow.
- `CHANGELOG.md` — BREAKING entry.

## 5. Descriptor fields

Replacement `Fields` table for `Type: "feishu"`:

| Name | Kind | Required | Enum | Default | Help |
|---|---|---|---|---|---|
| `app_id` | `FieldString` | ✓ | — | — | Self-built app ID from the Feishu Open Platform console. |
| `app_secret` | `FieldSecret` | ✓ | — | — | App secret paired with `app_id`. |
| `domain` | `FieldEnum` | ✓ | `[feishu, lark]` | `feishu` | `feishu` = feishu.cn (CN). `lark` = larksuite.com (overseas). |
| `encrypt_key` | `FieldSecret` | ✗ | — | — | Only needed when "Encrypted Push" is enabled in the app console. |
| `default_chat_id` | `FieldString` | ✗ | — | — | Fallback chat_id for pushes with no inbound context. Required only if the user intends pure push. |

`DisplayName` changes from `"Feishu / Lark (Bot Webhook)"` to `"Feishu / Lark (Self-built App)"`. `Summary` changes from `"Outbound-only..."` to `"Bidirectional Feishu/Lark adapter via a self-built app long-connection."`.

## 6. Data flow

### Inbound

```
Feishu long-connection
  → larkws.Client receives event
    → event type == "im.message.receive_v1"?
        no  → SDK drops; we never see it
        yes → our handler callback:
                msg_type == "text"?
                  no  → log at debug; return
                  yes → parse content JSON: {"text":"..."}
                        strip @_user_N tokens (regex: @_user_\d+\s*)
                        in := gateway.IncomingMessage{
                            Platform:  "feishu",
                            UserID:    event.Sender.SenderID.OpenID,
                            ChatID:    event.Message.ChatID,
                            Text:      cleanedText,
                            MessageID: event.Message.MessageID,
                            Timestamp: time.UnixMilli(eventTimeMs),
                        }
                        out, err := handler(ctx, in)
                        err != nil → log at warn; return
                        out == nil → return
                        out != nil → SendReply(ctx, *out)
                                     SendReply err → log at warn; return
```

### Outbound

```
SendReply(ctx, out):
  target := out.ChatID
  if target == "" { target = m.defaultChatID }
  if target == "" {
      return fmt.Errorf("feishu: no target chat_id (out.ChatID empty and default_chat_id not set)")
  }
  return m.sender.Create(ctx, target, out.Text)
```

`sender.Create` POSTs `im/v1/messages` with `msg_type="text"`, `receive_id_type="chat_id"`, `content=json.Marshal({"text":out.Text})`.

## 7. Error handling

| Site | Failure | Behavior |
|---|---|---|
| `NewFeishuApp` construction | `app_id` or `app_secret` empty in opts | `Build` returns `fmt.Errorf("feishu: missing app_id or app_secret")` — matches `mattermost_ws.go:61-63` style. |
| `NewFeishuApp` construction | `opts["webhook_url"] != "" && opts["app_id"] == ""` | Return `fmt.Errorf("feishu: webhook_url is no longer supported; migrate to a self-built app, see CHANGELOG")`. Surfaces at gateway startup, not at first message. |
| `Run` | long-connection drops | SDK reconnects internally with backoff. We do not wrap. |
| Event decode | malformed JSON or missing fields | Log at warn with raw event dump (truncated), return from handler. Do not kill the stream. |
| `handler(ctx, in)` returns error | — | Log at warn; no reply sent. |
| `SendReply` | no target | Return error (bubbles to caller). |
| `SendReply` | SDK call error (network / 4xx / 5xx) | Wrap `fmt.Errorf("feishu: send failed: %w", err)`. |
| `ctx` cancel | any time | `larkws.Client.Start(ctx)` returns; `Run` returns `ctx.Err()`. |

## 8. Concurrency

- One `FeishuApp` instance, one `Run` goroutine. Same pattern as `mattermost_ws`, `telegram`.
- `SendReply` may be called concurrently from multiple goroutines (e.g., two inbound events processed in parallel). The SDK's `lark.Client` HTTP layer is safe for concurrent use; the wrapping `feishuSDKSender` holds no mutable state.
- Dedup is done in `gateway/dedup.go` at the gateway layer using `IncomingMessage.MessageID`. We fill it; we do not re-implement dedup in the adapter.

## 9. Testing strategy

Two dependency seams let us fake the SDK:

```go
type feishuEventStream interface {
    Start(ctx context.Context) error
}
type feishuMessageSender interface {
    Create(ctx context.Context, chatID, text string) error
}
```

Production: `feishuSDKStream`, `feishuSDKSender`. Tests: hand-written fakes.

Test list (one task per test, TDD):

1. `TestFeishuApp_MissingCreds` — `NewFeishuApp(opts{})` → error.
2. `TestFeishuApp_WebhookURLSurfaced` — `NewFeishuApp(opts{"webhook_url":"https://..."})` → specific migration error.
3. `TestFeishuApp_IncomingText` — fake stream delivers one text event → handler called with expected `IncomingMessage`.
4. `TestFeishuApp_StripsAtMention` — event content `"@_user_1 hello"` → handler receives `Text:"hello"`.
5. `TestFeishuApp_IgnoresNonText` — event with `msg_type:"image"` → handler not called.
6. `TestFeishuApp_SendReplyToSource` — `SendReply(out{ChatID:"oc_a"})` → fake sender records `("oc_a", "text")`.
7. `TestFeishuApp_SendReplyFallback` — `SendReply(out{ChatID:""})` with `defaultChatID="oc_default"` → fake records `("oc_default", "text")`.
8. `TestFeishuApp_SendReplyNoTarget` — `SendReply(out{ChatID:""})` with empty default → error mentions `"no target"`.
9. `TestFeishuApp_ContextCancels` — fake stream returns when ctx cancels; `Run` returns.
10. `TestFeishuApp_HandlerErrorDoesNotKillStream` — handler returns error → next event still delivered.
11. `TestDescriptorFeishu_Fields` — registry lookup, Fields length + each field's Name/Kind/Required/Enum asserts.
12. `TestBuildFeishuApp_ViaRegistry` — call `Descriptor.Build` with valid opts → returns a `*FeishuApp` whose `Name() == "feishu"`.

Not tested:
- SDK internals (reconnect, token refresh, encrypt/decrypt) — SDK's responsibility.
- End-to-end against real Feishu — no CI credentials.

Parity test `descriptor_parity_test.go` auto-covers the new descriptor (no new wiring needed there).

## 10. Migration and breaking change

This is a hard break. Plan must land:

- **CHANGELOG entry under a `### Breaking` header** naming the removed `webhook_url` field and the required new fields.
- **Startup error** when `opts["webhook_url"]` is set on a `feishu` instance without `app_id`. Surfaces immediately on gateway startup (see §7).
- **`docs/smoke/feishu-app.md`** with the manual verification flow:
  1. Create self-built app in Feishu Open Platform.
  2. Under "Event & Callback → Events", subscribe to `im.message.receive_v1`.
  3. Under "Event & Callback → Subscription Mode", enable "Use long-connection to receive".
  4. Under "Permissions", grant `im:message` and `im:message.group_at_msg:readonly` (group @mention).
  5. Configure `gateway.platforms.feishu_main.type=feishu` with `app_id`, `app_secret`, `domain=feishu`.
  6. Restart gateway; confirm log shows `feishu: connected` (or equivalent SDK log).
  7. DM the bot → verify handler fires; expect reply at the source chat.
  8. @mention the bot in a group → verify `@_user_1` stripped and reply lands in the group.
- **`go.mod`** gets `github.com/larksuite/oapi-sdk-go/v3` added. Standalone `chore(deps):` commit separate from code changes so review is easy.
- **Rollback** is `git revert` of the merge; framework layer (registry, descriptor types, PlatformConfig) is untouched so the revert is clean.

## 11. Out-of-scope follow-ups

Noted but deferred:

- `msg_type="image"` / `file` / `sticker` inbound handling (needs a richer `IncomingMessage` than `Text`).
- Card actions.
- `base_url` override field for private-cloud Feishu deployments.
- Proactive outbound helper (send without an inbound trigger) beyond `default_chat_id`.

---

## Approval

Awaiting user review. On approval, the next step is the writing-plans skill to produce a task-by-task implementation plan against this spec.
