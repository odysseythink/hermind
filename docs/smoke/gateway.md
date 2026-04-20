# Gateway platform smoke flows

## Telegram proxy

Prerequisites: a local SOCKS5 proxy on 127.0.0.1:7890. Any of these
work: Clash, mihomo, v2rayN, Shadowsocks. The test assumes the proxy
has outbound reachability to api.telegram.org.

1. Add a Telegram platform instance via the web UI or config.yaml:

   ```yaml
   gateway:
     platforms:
       my_telegram:
         type: telegram
         options:
           token: "<your bot token from @BotFather>"
           proxy: socks5://127.0.0.1:7890
   ```

2. Start hermind:

   ```bash
   hermind gateway
   ```

   Expected: platform boots clean; no "connection timeout" on the
   initial reachability probe. If the proxy is mis-configured, the
   error surfaces with "telegram: probe failed" or
   "telegram: probe returned <status>".

3. Trigger a reachability test via the web UI or curl:

   ```bash
   curl -X POST http://127.0.0.1:9119/api/platforms/my_telegram/test \
        -H "Authorization: Bearer <token>"
   ```

   Expected: `{"ok": true}`. The request routes through the local proxy.

4. Send a message to the bot from a Telegram client. Verify the bot
   responds (polling + send both go through the proxy).

5. Kill the local proxy process. Restart hermind. Expected: the
   reachability probe fails with "connect: connection refused"
   (confirming the proxy IS being used — if it were bypassing, the
   probe would still succeed against the real api.telegram.org).

6. Remove the `proxy:` line from config. Restart. Expected: direct
   connection resumes. (This step only validates on a network
   without an outbound block — run it from outside mainland China.)

### Proxy URL formats

All three schemes are supported:

- HTTP proxy: `http://127.0.0.1:7890`
- HTTPS proxy: `https://proxy.corp.example:443`
- SOCKS5: `socks5://127.0.0.1:1080`
- SOCKS5 with credentials: `socks5://user:pass@host:port`

Invalid schemes (e.g. `ftp://`, `socks4://`) return a config error at
startup: `telegram: unsupported proxy scheme "<scheme>" (want
http/https/socks5)`.
