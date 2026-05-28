# OAuth Skills — Single-User Mode Only

**Date**: 2026-05-27
**Status**: Adopted
**Context**: gmail / google-calendar / outlook all hold credentials that grant access to a real human mailbox/calendar. Node disables them in `MULTI_USER_MODE=true` for the same reason.

**Decision**: All three CheckFn return false when `cfg.MultiUserMode == true`. Multi-user OAuth (per-user credentials, per-user tokens) is out of scope for PR-AR-7.
