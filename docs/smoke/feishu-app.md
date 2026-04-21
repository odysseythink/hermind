# Feishu self-built app smoke flow

Manual verification that the `feishu` adapter connects via long-connection
and round-trips a text message.

## 1. Create a self-built app

1. Open <https://open.feishu.cn/app> and create a custom app.
2. Grab **App ID** and **App Secret** from "Credentials & Basic Info".

## 2. Enable long-connection + permissions

1. Under **Event & Callback → Events**, subscribe to `im.message.receive_v1`.
2. Under **Event & Callback → Subscription Mode**, switch to
   "Use long-connection to receive events".
3. Under **Permissions & Scopes**, grant at least:
   - `im:message` (send messages)
   - `im:message.group_at_msg:readonly` (receive @mentions in groups)
   - `im:message.p2p_msg:readonly` (receive DMs)
4. If you enable **Encrypt Push**, copy the encrypt key.
5. Create a version and release internally.

## 3. Configure hermind

Example `config.yaml`:

```yaml
gateway:
  platforms:
    feishu_main:
      type: feishu
      options:
        app_id: "cli_xxxxxxxxxxxx"
        app_secret: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
        domain: "feishu"            # use "lark" for overseas Lark
        # encrypt_key: "..."        # only if Encrypt Push is enabled
        # default_chat_id: "oc_xxx" # only for pure-push scenarios
```

Restart `hermind`. Tail the log and look for an SDK line like
`ws client: connect success`.

## 4. Exercise

1. DM the bot from your Feishu account. Expect the app's handler to be
   invoked (gateway log shows inbound) and a reply delivered back in the DM.
2. Add the bot to a group and @mention it. Expect the `@_user_N` token to
   be stripped from the inbound text, and the reply to land in the group.
3. For pure push, omit `default_chat_id` and send an `OutgoingMessage` with
   an empty `ChatID` — expect an error from the gateway layer. Then set
   `default_chat_id` to a chat you control and retry; expect delivery.

## 5. Negative check — legacy config

Put `webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"`
under a `feishu` instance's options (no `app_id`). Restart. Expect a
startup error mentioning "webhook_url is no longer supported".
