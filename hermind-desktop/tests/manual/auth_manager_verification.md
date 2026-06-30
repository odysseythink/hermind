# AuthManager 手动验证记录

- 日期：2026-06-30
- 后端：`go run -tags="fts5" ./cmd/server/`，单用户模式，端口 3001
- 测试程序：`hermind-desktop/tests/auth/release/auth_manager_live_test.exe`

## 验证项

1. `login()` 在单用户模式下直接获取 token 并进入 `Authenticated` —— 通过
2. Token 被写入 `SettingsStore`（`auth/token` key）—— 通过
3. `restoreSession()` 从 SettingsStore 读取 token 并恢复 `Authenticated` 状态 —— 通过
4. `logout()` 清除 token 并进入 `Unauthenticated` —— 通过

## 测试输出

```
********* Start testing of TestAuthManagerLive *********
PASS   : TestAuthManagerLive::initTestCase()
  Token: eyJhbGciOiJIUzI1NiIsInR5cCI6Ik...
PASS   : TestAuthManagerLive::singleUserLoginSucceeds()
PASS   : TestAuthManagerLive::logoutClearsState()
  Token persisted: eyJhbGciOiJIUzI1NiIsInR5cCI6Ik...
PASS   : TestAuthManagerLive::restoreSessionWorks()
PASS   : TestAuthManagerLive::cleanupTestCase()
Totals: 5 passed, 0 failed, 0 skipped, 0 blacklisted
```

## 结论

AuthManager 0.2 功能与真实后端交互正常。单用户模式下：
- 登录成功获取 JWT token
- Token 持久化到 QSettings
- 登出清理所有状态
- 会话恢复正常工作
