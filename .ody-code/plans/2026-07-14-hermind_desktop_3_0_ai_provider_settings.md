# Hermind Desktop 阶段 3.0 — AI Provider Settings 框架实现计划（Index）

**Goal:** 在 `hermind-desktop` 中落地阶段 3 的原生设置框架：一个带 7 个 Tab（LLM / Vector Database / Embedding / Text Splitter / Audio / Transcription / Model Routers）二级导航的 `AiProviderSettingsWidget`，通过 `NavigationManager` 路由可达，提供 `setTabWidget()` 注册接口供子阶段 3.1–3.7 注入原生页或 WebView 兜底页。

**Architecture:** 完全复刻阶段 2.0 已验证的 `WorkspaceSettingsWidget` 模式：左侧 260px 侧边栏（`SidebarMenuButton` + `QButtonGroup`）+ 右侧 `QStackedWidget`（每 Tab 一个占位页，`setTabWidget()` 替换）。路由复用既有 `NavigationRoute.settingsPath` 字段，约定新前缀 `settings/ai-provider/<tab-id>`；`MainWindow` 将裸 `GeneralSettings` 导航（`settingsPath` 为空）保持显示旧 `MainSettingWidget` 以维持现有行为不变，仅匹配前缀时切到新框架。角色 gating 遵循 Web 端 `roles: ["admin"]` + 单机模式（空 role）全可见的规则。

**Tech Stack:** Qt 6 Widgets + qmake，C++17，Qt Test（headless）。

> For executing workers: implement this plan task-by-task (prefer a fresh subagent/Task per task — a clean context per task avoids single-session degradation). Steps use - [ ] checkboxes for tracking.

## File Structure

| 责任 | 文件 | Part |
|------|------|------|
| Tab 注册表（7 项 id/title） | 新建 `hermind-desktop/widgets/ai_provider_settings_tab.{h,cpp}` | widget.md |
| 框架控件（侧边栏 + 内容栈 + gating） | 新建 `hermind-desktop/widgets/ai_provider_settings_widget.{h,cpp}` | widget.md |
| 注册表测试 | 修改 `hermind-desktop/tests/widgets/tst_widgets.cpp`、`tests/widgets/widgets_test.pro` | widget.md |
| 框架控件测试 | 新建 `hermind-desktop/tests/widgets/tst_ai_provider_settings_widget.cpp`、`tests/widgets/ai_provider_settings_widget_test.pro` | widget.md |
| 页槽注册 + settingsPath 分发 | 修改 `hermind-desktop/mainwindow.cpp` | integration.md |
| MainWindow 集成测试 | 修改 `hermind-desktop/tests/mainwindow/tst_mainwindow.cpp`、`mainwindow_test.pro` | integration.md |
| 构建集成 | 修改 `hermind-desktop/hermind-desktop.pro`、`hermind-desktop/tests/run_tests.sh` | integration.md |

## Dependency Overview

```
Part 1 (widget.md):
  Task 1: AiProviderSettingsTabs 注册表        (依赖: 无)
     |
     v
  Task 2: AiProviderSettingsWidget 框架控件     (依赖: Task 1)

Part 2 (integration.md):
  Task 3: MainWindow 页槽注册 + 路由分发        (依赖: widget.md: Task 2)
     |
     v
  Task 4: 构建集成 + 全量测试                   (依赖: Task 3)
```

可并行：无（Task 2 引用 Task 1 的注册表；Task 3 引用 Task 2 的控件类型；Task 4 依赖全部）。

## Risks & Open Questions

| # | Risk | Mitigation |
|---|------|------------|
| R1 | 将 `GeneralSettings` 页槽换成新框架后，现有裸导航（聊天页底部设置按钮、AgentConfig "Configure Agent Skills"）与测试 `generalSettingsPageIsReachable`（断言 index==1）回归 | Task 3 保持 `registerPage(GeneralSettings, settingWidget)` 不变，仅当 `settingsPath` 匹配 `settings/ai-provider/` 前缀时才 `setCurrentWidget` 覆盖；Task 3 新增 `bareGeneralSettingsRouteShowsLegacyPage` 回归测试锁定旧行为 |
| R2 | 单机模式下 `AuthManager::currentUser().role()` 为空字符串，若按 "非 admin 即隐藏" 会导致单机用户看不到任何 Tab | `setUserRole` 将空 role 视为完全可见（与 Web 端 `Option` 组件 `!user` 时全部展示一致）；测试 `buttonsVisibleByDefaultForSingleUser` 锁定 |
| R3 | Tab id `vector-database` 与 `WorkspaceSettingsTabs` 的同名 id 冲突 | 两者是独立注册表类（`AiProviderSettingsTabs` vs `WorkspaceSettingsTabs`），objectName `tabButton_*` 作用域各自 widget 内，无符号或查找冲突 |

## Spec coverage

| Requirement（roadmap 3.0 + 验收标准） | Task(s) | Status |
|------|--------|--------|
| 左侧菜单 + 右侧内容的二级导航框架，7 个 AI 提供商 Tab | Task 1, Task 2 | covered |
| Tab 注册接口供 3.1–3.7 注入原生/WebView 页（`setTabWidget`） | Task 2 | covered |
| 依赖 0.4 NavigationManager：路由可达、返回上级 | Task 3 | covered |
| 依赖 0.5 通用控件（SidebarMenuButton/IconButton/ThemeColors）与主题跟随 | Task 2 | covered |
| 角色 gating（Web 端 `roles: ["admin"]`，单机模式全可见） | Task 2 | covered |
| 独立可验证：编译通过 + 自动化验收（9 项控件测试 + 3 项集成测试 + 5 项注册表测试） | Task 1–4 | covered |
| "封装 WebView 页与原生页的统一加载接口" | — | no-op：统一接口即 `setTabWidget()`（原生页与 `QWebEngineView` 包装控件同为 QWidget）；`QWebChannel` 桥接协议按 roadmap 由 3.1 [design] 子阶段定义并经审批，3.0 不预先实现 |
| 3.1–3.7 的页面内容（LLM/Embedding/VectorDB/TextSplitter/Audio/Transcription/Routers） | — | no-op：属后续子阶段，3.0 仅交付框架与占位页 |

## Self-review

- [x] 1. Spec-coverage table: roadmap 3.0 行（框架/原生框架/[plan]/依赖 0.0,0.4,0.5）与验收标准全部映射为 covered；WebView 桥接与后续页面内容标 no-op 并给出理由；无 GAP。
- [x] 2. Placeholder scan: 无 TODO/TBD/FIXME；widget.md 与 integration.md 中所有实现与测试均给出完整代码；内容占位页是有意的交付物（3.1–3.7 经 `setTabWidget()` 替换）。
- [x] 3. No phantom tasks: 4 个任务各自产生可验证变更（注册表+5 项测试 / 控件+9 项测试 / 路由+3 项集成测试 / 构建+全量验证），无 "already done" 或空任务。
- [x] 4. Dependency soundness: Task 2 仅用 Task 1 的 `AiProviderSettingsTabs`；Task 3 仅用 widget.md Task 2 的 `AiProviderSettingsWidget`（`currentTabId()`/`setActiveTab()`/`returnClicked`）；Task 4 依赖 Task 3 的 mainwindow.cpp 改动；无符号先于定义被引用。
- [x] 5. Caller & build soundness: 全计划未修改任何共享签名/类型/结构字段（只新增两个类 + `onCurrentRouteChanged` 私有槽内部分支）。运行时消费者 `MainWindow::onCurrentRouteChanged` 已逐值追踪：裸 `GeneralSettings`（settingsPath 空）→ `pageIndex` 返回旧页 index 1 并 return，与现有测试 `generalSettingsPageIsReachable` 的断言一致；`settings/ai-provider/audio-preference` → 前缀匹配 → `setCurrentWidget(aiWidget)` + `setActiveTab("audio-preference")`，与 Task 3 测试断言一致。Task 4 以主工程 `qmake && make` 全量构建 + `tests/run_tests.sh` 全量测试收尾（Qt 工程的 whole-tree 验证）。
- [x] 6. Test-the-risk: 状态突变均有行为测试：角色 gating（`buttonsHiddenForMemberRole` 等）、路由分发（`aiProviderRouteShowsAiProviderFrame`）。must-survive 断言与实现常量逐项核对：单机空 role 可见 ↔ `role.isEmpty() || role == "admin"`；7 个 Tab id 断言 ↔ 注册表常量；裸 GeneralSettings 显示旧页 ↔ 前缀不匹配时不覆盖 `setCurrentIndex`。
- [x] 7. Type consistency: integration.md 引用的 `AiProviderSettingsWidget::currentTabId()`、`setActiveTab(const QString &)`、`returnClicked` 信号与 widget.md Task 2 头文件声明完全一致；`NavigationRoute.settingsPath` 为既有 QString 字段，未改动。

## Out-of-scope

- `MainSettingWidget`（旧外观设置页）：保留为裸 `GeneralSettings` 路由的显示页，其 9 个菜单按钮仍只打印 `qDebug`；阶段 4.0/5.0 落地后再评估替换，本子阶段不改动。
- `NavigationPage::AdminSettings` 页槽：仍未注册（`qWarning` 分支），属阶段 4.0。
- `widgets/vector_database_tab.{h,cpp}`：这是阶段 2.3 的**工作区级** Vector Database Tab，与 3.4 的**全局** `/settings/vector-database`（WebView 兜底）是不同概念；不动。
- `widgets/llm_provider_info.*`、`llm_model_selector.*`：阶段 2.2 聊天设置的 LLM 选择控件，非 `/settings/llm-preference` 页面；不动。
- WebView（`QWebEngineView`/`QWebChannel`）：`hermind-desktop.pro` 暂不新增 `webenginewidgets` 模块；由 3.1 [design] 决定。
- i18n 运行时语言切换：标题用 `tr()` 包裹即可，运行时切换属阶段 7.4。

## Assumptions

- Assumption: settingsPath 约定为 `settings/ai-provider/<tab-id>`（repo 内无既有约定；tab-id 与 Web 路由末段一致，3.1–3.7 沿用）。
- Assumption: 裸 `GeneralSettings` 导航继续显示旧 `MainSettingWidget`，直到阶段 4.0/5.0 框架落地后统一替换。
- Assumption: 单机模式（空 role）对 AI 提供商设置拥有完全访问权，与 Web 端行为一致。
- Assumption: 3.0 不引入 `QWebEngineView`/`QWebChannel`；统一页面加载接口为 `setTabWidget()`，桥接协议由 3.1 [design] 定义。

## Parts
| # | File | Scope | Status |
|---|---|---|---|
| 1 | `2026-07-14-hermind_desktop_3_0_ai_provider_settings/widget.md` | Tab 注册表 + 框架控件（Task 1-2） | done |
| 2 | `2026-07-14-hermind_desktop_3_0_ai_provider_settings/integration.md` | MainWindow 路由分发 + 构建集成（Task 3-4） | done |
