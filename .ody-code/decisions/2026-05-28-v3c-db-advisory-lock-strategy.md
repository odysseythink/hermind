# v3-C — OAuth Refresh DB Advisory Lock

**Date**: 2026-05-28  
**Status**: Adopted

PR-AR-7 used `sync.Mutex` to serialize `ValidAccessToken`, which is only effective within a single process. v3-C replaces it with GORM `clause.Locking{Strength:"UPDATE"}` + Transaction. On PostgreSQL/MySQL this becomes a true row lock; SQLite's single-writer mode naturally serializes. The `sync.Mutex` is retained as a fast-path within the process (harmless).
