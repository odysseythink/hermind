# Hermind Desktop 阶段 3.0 — Part 2: MainWindow 路由分发 + 构建集成

**Scope:** 将 Part 1 产出的 `AiProviderSettingsWidget` 挂入 `MainWindow` 的 `QStackedWidget`，在 `onCurrentRouteChanged` 中按 `settingsPath` 前缀 `settings/ai-provider/` 分发到新框架；裸 `GeneralSettings` 导航保持显示旧 `MainSettingWidget`。最后把新源文件接入主工程与测试脚本并做全量验证。

## Task 3: MainWindow 页槽注册与 settingsPath 分发

**Depends on:** widget.md: Task 2（`AiProviderSettingsWidget` 类型与 `setActiveTab()`）

**Files:**
- Modify: `hermind-desktop/mainwindow.cpp`（约 +25 行）
- Test: `hermind-desktop/tests/mainwindow/tst_mainwindow.cpp`、`hermind-desktop/tests/mainwindow/mainwindow_test.pro`

路由约定：`NavigationRoute{ page: GeneralSettings, settingsPath: "settings/ai-provider/<tab-id>" }` 显示新框架并选中 `<tab-id>`；`settingsPath` 为空或不匹配该前缀时仍显示旧 `MainSettingWidget`（现有行为来源：`mainwindow.cpp:36` 聊天页底部设置按钮、`mainwindow.cpp:82` AgentConfig 的 "Configure Agent Skills" 均走裸 `GeneralSettings`）。

- [ ] 在 `tst_mainwindow.cpp` 追加失败测试。文件头部 include 区追加：
  ```cpp
  #include "ai_provider_settings_widget.h"
  #include "main_setting_widget.h"
  ```
  （`QToolButton` 已在文件头部 include。）在 `TestMainWindow` 的 `private slots:` 追加三个声明：
  ```cpp
  void aiProviderRouteShowsAiProviderFrame();
  void bareGeneralSettingsRouteShowsLegacyPage();
  void aiProviderReturnButtonGoesBack();
  ```
  实现：
  ```cpp
  void TestMainWindow::aiProviderRouteShowsAiProviderFrame()
  {
      MainWindow w;
      auto *stack = w.findChild<QStackedWidget *>();
      QVERIFY(stack);

      NavigationRoute route;
      route.page = NavigationPage::GeneralSettings;
      route.settingsPath = QStringLiteral("settings/ai-provider/audio-preference");
      NavigationManager::instance().navigateTo(route);

      auto *aiWidget = w.findChild<AiProviderSettingsWidget *>(
          QStringLiteral("aiProviderSettingsWidget"));
      QVERIFY(aiWidget);
      QCOMPARE(stack->currentWidget(), static_cast<QWidget *>(aiWidget));
      QCOMPARE(aiWidget->currentTabId(), QStringLiteral("audio-preference"));
  }

  void TestMainWindow::bareGeneralSettingsRouteShowsLegacyPage()
  {
      MainWindow w;
      auto *stack = w.findChild<QStackedWidget *>();
      QVERIFY(stack);

      NavigationRoute route;
      route.page = NavigationPage::GeneralSettings;
      NavigationManager::instance().navigateTo(route);

      auto *legacy = w.findChild<MainSettingWidget *>();
      QVERIFY(legacy);
      QCOMPARE(stack->currentWidget(), static_cast<QWidget *>(legacy));
  }

  void TestMainWindow::aiProviderReturnButtonGoesBack()
  {
      MainWindow w;

      NavigationRoute aiRoute;
      aiRoute.page = NavigationPage::GeneralSettings;
      aiRoute.settingsPath = QStringLiteral("settings/ai-provider/llm-preference");
      NavigationManager::instance().navigateTo(aiRoute);

      auto *aiWidget = w.findChild<AiProviderSettingsWidget *>(
          QStringLiteral("aiProviderSettingsWidget"));
      QVERIFY(aiWidget);
      auto *returnButton = aiWidget->findChild<QToolButton *>(
          QStringLiteral("returnButton"));
      QVERIFY(returnButton);

      QTest::mouseClick(returnButton, Qt::LeftButton);

      QCOMPARE(NavigationManager::instance().currentPage(),
               NavigationPage::DefaultChat);
  }
  ```
  并在 `mainwindow_test.pro` 的 `SOURCES` 追加：
  ```
      ../../widgets/ai_provider_settings_widget.cpp \
      ../../widgets/ai_provider_settings_tab.cpp \
  ```
  `HEADERS` 追加：
  ```
      ../../widgets/ai_provider_settings_widget.h \
      ../../widgets/ai_provider_settings_tab.h \
  ```
- [ ] 运行并验证 FAILS：
  ```bash
  cd hermind-desktop/tests/mainwindow && qmake mainwindow_test.pro && make && QT_QPA_PLATFORM=offscreen ./tst_mainwindow
  ```
  预期：`findChild<AiProviderSettingsWidget *>` 返回 null，3 个新测试失败。
- [ ] 修改 `mainwindow.cpp`：
  1. 头部 include 区新增：
     ```cpp
     #include "ai_provider_settings_widget.h"
     ```
  2. 构造函数中 `registerPage(NavigationPage::GeneralSettings, settingWidget);` 所在 `if (settingWidget) { ... }` 块之后追加：
     ```cpp
     auto *aiProviderWidget = new AiProviderSettingsWidget(
         AuthManager::instance().apiClient(), this);
     aiProviderWidget->setObjectName(QStringLiteral("aiProviderSettingsWidget"));
     ui->stackedWidget->addWidget(aiProviderWidget);
     connect(aiProviderWidget, &AiProviderSettingsWidget::returnClicked,
             this, []() {
         NavigationManager::instance().goBack();
     });
     ```
     保持 `registerPage(NavigationPage::GeneralSettings, settingWidget)` 不变（页槽仍注册旧页，裸 GeneralSettings 导航行为与现有测试 `generalSettingsPageIsReachable` / `agentSkillsRequestedNavigatesToGeneralSettings` 完全一致）。
  3. `onCurrentRouteChanged` 中 `ui->stackedWidget->setCurrentIndex(index);` 之后、`if (route.page == NavigationPage::WorkspaceSettings)` 分支之前插入：
     ```cpp
     if (route.page == NavigationPage::GeneralSettings) {
         const QString prefix = QStringLiteral("settings/ai-provider/");
         if (route.settingsPath.startsWith(prefix)) {
             auto *aiWidget = findChild<AiProviderSettingsWidget *>(
                 QStringLiteral("aiProviderSettingsWidget"));
             if (aiWidget) {
                 ui->stackedWidget->setCurrentWidget(aiWidget);
                 aiWidget->setActiveTab(route.settingsPath.mid(prefix.size()));
             }
         }
         return;
     }
     ```
- [ ] 重新构建运行并验证 PASSES：
  ```bash
  cd hermind-desktop/tests/mainwindow && qmake mainwindow_test.pro && make && QT_QPA_PLATFORM=offscreen ./tst_mainwindow
  ```
  预期：新旧测试全部通过；重点确认原有 `generalSettingsPageIsReachable`（断言 `currentIndex() == 1`）与 `agentSkillsRequestedNavigatesToGeneralSettings` 无回归。
- [ ] Commit：
  ```bash
  git add hermind-desktop/mainwindow.cpp hermind-desktop/tests/mainwindow/tst_mainwindow.cpp hermind-desktop/tests/mainwindow/mainwindow_test.pro
  git commit -m "feat(desktop): route AI provider settings paths to new frame (3.0)"
  ```

## Task 4: 构建集成与全量验证

**Depends on:** Task 3（mainwindow.cpp 已引用新控件，主工程必须同时编译新源文件）

**Files:**
- Modify: `hermind-desktop/hermind-desktop.pro`
- Modify: `hermind-desktop/tests/run_tests.sh`

- [ ] `hermind-desktop.pro` 的 `SOURCES` 列表（`widgets/workspace_settings_widget.cpp` 行附近）追加：
  ```
      widgets/ai_provider_settings_tab.cpp \
      widgets/ai_provider_settings_widget.cpp \
  ```
  `HEADERS` 列表对应追加：
  ```
      widgets/ai_provider_settings_tab.h \
      widgets/ai_provider_settings_widget.h \
  ```
- [ ] `tests/run_tests.sh` 在 widgets 段 `run_test widgets workspace_settings_widget_test.pro tst_workspace_settings_widget` 行之后追加：
  ```bash
  run_test widgets ai_provider_settings_widget_test.pro tst_ai_provider_settings_widget
  ```
- [ ] 全量构建：
  ```bash
  cd hermind-desktop && qmake hermind-desktop.pro && make -j8
  ```
  （Windows llvm-mingw 工具链用 `mingw32-make`。）预期：编译链接通过，无新警告。
- [ ] 全量测试：
  ```bash
  cd hermind-desktop/tests && QT_QPA_PLATFORM=offscreen ./run_tests.sh
  ```
  预期：输出末尾 `=== all desktop unit tests passed ===`；新增的 `tst_ai_provider_settings_widget` 9 项通过，`tst_mainwindow`、`widgets_test` 原有测试无回归。
- [ ] 手动验证：`make dev` 等价地启动 `release/hermind-desktop`（或 `./hermind-desktop`），通过代码注入或后续入口导航到 `settings/ai-provider/audio-preference`，确认左侧出现 7 个 Tab、右侧显示占位文案、返回键回到聊天页。（本阶段尚无 UI 入口按钮，入口由 3.1+ 或阶段 4/5 设置总菜单接入；自动化测试已覆盖路由分发。）
- [ ] Commit：
  ```bash
  git add hermind-desktop/hermind-desktop.pro hermind-desktop/tests/run_tests.sh
  git commit -m "chore(desktop): wire AI provider settings frame into build and test suite (3.0)"
  ```

## Self-review (Part 2)

- [ ] 1. Spec-coverage: 本 Part 覆盖的 spec 项（NavigationManager 路由可达、返回上级、构建集成、全量验收）全部实现；index 中 no-op 项（WebView 桥接、3.1–3.7 页面内容）不在本 Part 范围。
- [ ] 2. Placeholder scan: 无 TODO/TBD；测试与 mainwindow.cpp 改动均给出完整代码。
- [ ] 3. No phantom tasks: Task 3 产出可验证的路由行为（3 项集成测试）；Task 4 产出主工程构建 + 全量测试验证。
- [ ] 4. Dependency soundness: Task 3 的 `AiProviderSettingsWidget` 类型来自 widget.md: Task 2（已完成）；Task 4 依赖 Task 3 的 mainwindow.cpp 改动。无反向引用。
- [ ] 5. Caller & build soundness: 未修改任何共享签名（`onCurrentRouteChanged` 为私有槽内部分支新增；`NavigationRoute` 结构未变）。路由消费者 `MainWindow::onCurrentRouteChanged` 已逐值确认：裸 GeneralSettings → `pageIndex` 返回旧页 index 1 并直接 return；`settings/ai-provider/<id>` → 先 `setCurrentIndex(1)` 再 `setCurrentWidget(aiWidget)` 覆盖。Task 4 以主工程全量构建 + `run_tests.sh` 全量测试收尾（Qt 工程无 `cargo check --workspace` 等价物，`qmake && make` 全量构建即 whole-tree typecheck）。
- [ ] 6. Test-the-risk: 路由分发（状态突变）有行为测试 `aiProviderRouteShowsAiProviderFrame`；旧行为 must-survive 场景由 `bareGeneralSettingsRouteShowsLegacyPage` 锁定，其断言（stack 当前页为 `MainSettingWidget`）与实现分支（前缀不匹配时不覆盖 `setCurrentIndex`）一致；返回键行为由 `aiProviderReturnButtonGoesBack` 锁定。
- [ ] 7. Type consistency: 测试引用的 `AiProviderSettingsWidget::currentTabId()`、`setActiveTab(const QString &)`、`returnClicked` 信号与 widget.md Task 2 头文件声明完全一致；`NavigationRoute.settingsPath` 为既有 QString 字段。
