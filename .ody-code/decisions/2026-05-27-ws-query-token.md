# WS Auth via Query Token (PR-AR-4)

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Browser WebSocket API can't send Authorization headers; need to ferry auth across upgrade.

## Options
- **Query string `?token=`**: ✓ adopted. Reuses PR-AR-1 WSValidatedRequest as-is.
- `Sec-WebSocket-Protocol` subprotocol: rejected — adds middleware complexity; not all proxies preserve the header.
- Cookie-based: rejected — would require fetching CSRF+session cookies from FE; mismatched with our Bearer-JWT auth design.

## Risk
Token logged in proxy access logs. Mitigation: 3-minute TTL + single-use (deleted on Validate). Window is ~5 seconds in practice (FE dials immediately after SSE chunk).
