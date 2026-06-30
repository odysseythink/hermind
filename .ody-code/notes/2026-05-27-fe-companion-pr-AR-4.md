# Frontend Companion PR for PR-AR-4

## Context
PR-AR-4 adds `websocketToken` to the `agentInitWebsocketConnection` SSE chunk. The Go backend now issues single-use 3-minute temporary tokens for WebSocket auth. The frontend must pass this token as a query parameter when dialing the WS.

## Required Changes (~4 lines)

### 1. `frontend/src/utils/chat/index.js` (~line 140)
Currently:
```js
setWebsocket(chatResult.websocketUUID);
```
Change to:
```js
setWebsocket(chatResult.websocketUUID);
setWebsocketToken(chatResult.websocketToken); // NEW
```

### 2. `frontend/src/components/WorkspaceChat/ChatContainer/index.jsx` (~line 281)
Currently:
```js
new WebSocket(`${websocketURI()}/api/agent-invocation/${socketId}`)
```
Change to:
```js
new WebSocket(`${websocketURI()}/api/agent-invocation/${socketId}?token=${websocketToken}`)
```

## Why
Browser WebSocket API cannot send `Authorization` headers. The token must travel via query string or `Sec-WebSocket-Protocol`. We chose query string to reuse the existing `WSValidatedRequest` middleware from PR-AR-1 without additional middleware complexity.

## Backward Compatibility
- Old backend (no `websocketToken` field): `websocketToken` will be `undefined`, and the WS dial will omit `?token=undefined` or fail. **This PR must land together with the FE companion PR.**
- New backend + old frontend: old frontend ignores `websocketToken` and dials WS without token → WS upgrade rejected with 401. **Hard dependency.**
