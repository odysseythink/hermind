# Hermind Desktop 阶段 3.0 — Part 1: Tab 注册表 + 框架控件

**Scope:** 新建 `AiProviderSettingsTabs` 注册表（7 个 AI 提供商 Tab）与 `AiProviderSettingsWidget` 框架控件（侧边栏二级导航 + 内容栈 + 角色 gating + 主题跟随），含全部单元测试。本 Part 产出 Task 3 依赖的控件类型与 `setTabWidget()` 注册接口。

## Task 1: AiProviderSettingsTabs 注册表

**Depends on:** none

**Files:**
- Create: `hermind-desktop/widgets/ai_provider_settings_tab.h`
- Create: `hermind-desktop/widgets/ai_provider_settings_tab.cpp`
- Test: `hermind-desktop/tests/widgets/tst_widgets.cpp`、`hermind-desktop/tests/widgets/widgets_test.pro`

Tab id 与顺序取自 `frontend/src/components/SettingsSidebar/index.jsx` 的 AI Providers 分组及 `frontend/src/utils/paths.js`（id 即路由末段，保证 3.1–3.7 的 settingsPath 与 Web 路由一一对应）。

- [ ] 编写失败测试。在 `tst_widgets.cpp` 的 `TestWidgets` 中追加（头部加 `#include "ai_provider_settings_tab.h"`）：
  ```cpp
  void aiProviderSettingsTabs_all_hasSevenTabs();
  void aiProviderSettingsTabs_orderMatchesWebSidebar();
  void aiProviderSettingsTabs_indexOfFindsTab();
  void aiProviderSettingsTabs_indexOfInvalidReturnsNegativeOne();
  void aiProviderSettingsTabs_titleOfFindsTitle();
  ```
  实现：
  ```cpp
  void TestWidgets::aiProviderSettingsTabs_all_hasSevenTabs()
  {
      QCOMPARE(AiProviderSettingsTabs::all().size(), 7);
  }

  void TestWidgets::aiProviderSettingsTabs_orderMatchesWebSidebar()
  {
      const auto &tabs = AiProviderSettingsTabs::all();
      QCOMPARE(tabs.at(0).id, QStringLiteral("llm-preference"));
      QCOMPARE(tabs.at(1).id, QStringLiteral("vector-database"));
      QCOMPARE(tabs.at(2).id, QStringLiteral("embedding-preference"));
      QCOMPARE(tabs.at(3).id, QStringLiteral("text-splitter-preference"));
      QCOMPARE(tabs.at(4).id, QStringLiteral("audio-preference"));
      QCOMPARE(tabs.at(5).id, QStringLiteral("transcription-preference"));
      QCOMPARE(tabs.at(6).id, QStringLiteral("model-routers"));
  }

  void TestWidgets::aiProviderSettingsTabs_indexOfFindsTab()
  {
      QCOMPARE(AiProviderSettingsTabs::indexOf(QStringLiteral("audio-preference")), 4);
  }

  void TestWidgets::aiProviderSettingsTabs_indexOfInvalidReturnsNegativeOne()
  {
      QCOMPARE(AiProviderSettingsTabs::indexOf(QStringLiteral("nope")), -1);
  }

  void TestWidgets::aiProviderSettingsTabs_titleOfFindsTitle()
  {
      QVERIFY(!AiProviderSettingsTabs::titleOf(QStringLiteral("llm-preference")).isEmpty());
      QVERIFY(AiProviderSettingsTabs::titleOf(QStringLiteral("nope")).isEmpty());
  }
  ```
- [ ] 在 `widgets_test.pro` 的 `SOURCES`/`HEADERS` 追加 `../../widgets/ai_provider_settings_tab.cpp` 与 `../../widgets/ai_provider_settings_tab.h`，运行并验证 FAILS：
  ```bash
  cd hermind-desktop/tests/widgets && qmake widgets_test.pro && make && QT_QPA_PLATFORM=offscreen ./widgets_test
  ```
  预期：链接失败（undefined `AiProviderSettingsTabs`）或编译失败。
- [ ] 编写实现（完整代码）。

  `ai_provider_settings_tab.h`：
  ```cpp
  #ifndef AI_PROVIDER_SETTINGS_TAB_H
  #define AI_PROVIDER_SETTINGS_TAB_H

  #include <QString>
  #include <QVector>

  struct AiProviderSettingsTab {
      QString id;
      QString title;
  };

  class AiProviderSettingsTabs
  {
  public:
      static const QVector<AiProviderSettingsTab> &all();
      static int indexOf(const QString &id);
      static QString titleOf(const QString &id);
  };

  #endif // AI_PROVIDER_SETTINGS_TAB_H
  ```

  `ai_provider_settings_tab.cpp`：
  ```cpp
  #include "ai_provider_settings_tab.h"

  #include <QObject>

  const QVector<AiProviderSettingsTab> &AiProviderSettingsTabs::all()
  {
      static const QVector<AiProviderSettingsTab> tabs = {
          { QStringLiteral("llm-preference"),           QObject::tr("LLM Preference") },
          { QStringLiteral("vector-database"),          QObject::tr("Vector Database") },
          { QStringLiteral("embedding-preference"),     QObject::tr("Embedding Preference") },
          { QStringLiteral("text-splitter-preference"), QObject::tr("Text Splitting & Chunking") },
          { QStringLiteral("audio-preference"),         QObject::tr("Voice & Speech") },
          { QStringLiteral("transcription-preference"), QObject::tr("Transcription Model") },
          { QStringLiteral("model-routers"),            QObject::tr("Model Routers") },
      };
      return tabs;
  }

  int AiProviderSettingsTabs::indexOf(const QString &id)
  {
      const auto &tabs = all();
      for (int i = 0; i < tabs.size(); ++i) {
          if (tabs.at(i).id == id)
              return i;
      }
      return -1;
  }

  QString AiProviderSettingsTabs::titleOf(const QString &id)
  {
      const int idx = indexOf(id);
      if (idx < 0)
          return QString();
      return all().at(idx).title;
  }
  ```
- [ ] 重新构建运行并验证 PASSES（同上命令，5 个新测试全绿）。
- [ ] Commit：
  ```bash
  git add hermind-desktop/widgets/ai_provider_settings_tab.h hermind-desktop/widgets/ai_provider_settings_tab.cpp hermind-desktop/tests/widgets/tst_widgets.cpp hermind-desktop/tests/widgets/widgets_test.pro
  git commit -m "feat(desktop): add AI provider settings tab registry (3.0)"
  ```

## Task 2: AiProviderSettingsWidget 框架控件

**Depends on:** Task 1

**Files:**
- Create: `hermind-desktop/widgets/ai_provider_settings_widget.h`（约 55 行）
- Create: `hermind-desktop/widgets/ai_provider_settings_widget.cpp`（约 230 行）
- Test: 新建 `hermind-desktop/tests/widgets/tst_ai_provider_settings_widget.cpp`、`hermind-desktop/tests/widgets/ai_provider_settings_widget_test.pro`

控件结构逐项对齐 `workspace_settings_widget.cpp`（侧边栏 260px、`IconButton` 返回键、`SidebarMenuButton` + `QButtonGroup` 独占、`QStackedWidget` 占位页、`ThemeManager::themeChanged` → `applyStyle`）。

- [ ] 编写失败测试 `tst_ai_provider_settings_widget.cpp`（完整代码）：
  ```cpp
  #include <QtTest/QtTest>
  #include <QSignalSpy>
  #include <QLabel>
  #include <QStackedWidget>

  #include "ai_provider_settings_widget.h"
  #include "ai_provider_settings_tab.h"
  #include "sidebar_menu_button.h"

  class TestAiProviderSettingsWidget : public QObject
  {
      Q_OBJECT
  private slots:
      void tabButtonsExistForAllSevenTabs();
      void defaultTabIsLlmPreference();
      void setActiveTabSwitchesStackAndEmitsTabChanged();
      void unknownTabFallsBackToFirst();
      void setTabWidgetReplacesPlaceholder();
      void buttonsVisibleByDefaultForSingleUser();
      void buttonsHiddenForMemberRole();
      void buttonsVisibleForAdminRole();
      void activeTabFallsBackWhenRoleRevoked();
  };

  void TestAiProviderSettingsWidget::tabButtonsExistForAllSevenTabs()
  {
      AiProviderSettingsWidget w(nullptr);
      for (const AiProviderSettingsTab &tab : AiProviderSettingsTabs::all()) {
          auto *btn = w.findChild<SidebarMenuButton *>(
              QStringLiteral("tabButton_") + tab.id);
          QVERIFY2(btn, qPrintable(tab.id));
      }
  }

  void TestAiProviderSettingsWidget::defaultTabIsLlmPreference()
  {
      AiProviderSettingsWidget w(nullptr);
      QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
  }

  void TestAiProviderSettingsWidget::setActiveTabSwitchesStackAndEmitsTabChanged()
  {
      AiProviderSettingsWidget w(nullptr);
      QSignalSpy spy(&w, &AiProviderSettingsWidget::tabChanged);
      w.setActiveTab(QStringLiteral("audio-preference"));
      QCOMPARE(w.currentTabId(), QStringLiteral("audio-preference"));
      QCOMPARE(spy.count(), 1);
      QCOMPARE(spy.takeFirst().at(0).toString(), QStringLiteral("audio-preference"));

      auto *stack = w.findChild<QStackedWidget *>(QStringLiteral("contentStack"));
      QVERIFY(stack);
      QCOMPARE(stack->currentIndex(),
               AiProviderSettingsTabs::indexOf(QStringLiteral("audio-preference")));

      auto *btn = w.findChild<SidebarMenuButton *>(
          QStringLiteral("tabButton_audio-preference"));
      QVERIFY(btn && btn->isChecked());
  }

  void TestAiProviderSettingsWidget::unknownTabFallsBackToFirst()
  {
      AiProviderSettingsWidget w(nullptr);
      w.setActiveTab(QStringLiteral("does-not-exist"));
      QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
  }

  void TestAiProviderSettingsWidget::setTabWidgetReplacesPlaceholder()
  {
      AiProviderSettingsWidget w(nullptr);
      auto *page = new QLabel(QStringLiteral("native page"));
      page->setObjectName(QStringLiteral("injectedPage"));
      w.setTabWidget(QStringLiteral("audio-preference"), page);
      w.setActiveTab(QStringLiteral("audio-preference"));

      auto *stack = w.findChild<QStackedWidget *>(QStringLiteral("contentStack"));
      QVERIFY(stack);
      QCOMPARE(stack->currentWidget(), static_cast<QWidget *>(page));
  }

  void TestAiProviderSettingsWidget::buttonsVisibleByDefaultForSingleUser()
  {
      // 单机模式：AuthManager 用户为空（role 为空字符串），AI 提供商页完全可见。
      AiProviderSettingsWidget w(nullptr);
      auto *btn = w.findChild<SidebarMenuButton *>(
          QStringLiteral("tabButton_llm-preference"));
      QVERIFY(btn);
      QVERIFY(!btn->isHidden());
  }

  void TestAiProviderSettingsWidget::buttonsHiddenForMemberRole()
  {
      AiProviderSettingsWidget w(nullptr);
      w.setUserRole(QStringLiteral("member"));
      auto *btn = w.findChild<SidebarMenuButton *>(
          QStringLiteral("tabButton_llm-preference"));
      QVERIFY(btn);
      QVERIFY(btn->isHidden());
  }

  void TestAiProviderSettingsWidget::buttonsVisibleForAdminRole()
  {
      AiProviderSettingsWidget w(nullptr);
      w.setUserRole(QStringLiteral("member"));
      w.setUserRole(QStringLiteral("admin"));
      auto *btn = w.findChild<SidebarMenuButton *>(
          QStringLiteral("tabButton_llm-preference"));
      QVERIFY(btn);
      QVERIFY(!btn->isHidden());
  }

  void TestAiProviderSettingsWidget::activeTabFallsBackWhenRoleRevoked()
  {
      AiProviderSettingsWidget w(nullptr);
      w.setActiveTab(QStringLiteral("model-routers"));
      QCOMPARE(w.currentTabId(), QStringLiteral("model-routers"));
      w.setUserRole(QStringLiteral("member"));
      QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
  }

  QTEST_MAIN(TestAiProviderSettingsWidget)
  #include "tst_ai_provider_settings_widget.moc"
  ```
- [ ] 新建 `ai_provider_settings_widget_test.pro`：完整复制 `workspace_settings_widget_test.pro` 的全部 INCLUDEPATH 与依赖 SOURCES/HEADERS，仅改四处：
  - `TARGET = tst_ai_provider_settings_widget`
  - 测试源 `tst_ai_provider_settings_widget.cpp`
  - 被测源 `../../widgets/ai_provider_settings_widget.cpp` + `../../widgets/ai_provider_settings_tab.cpp`（替换 `workspace_settings_widget.cpp` / `workspace_settings_tab.cpp`）
  - HEADERS 同理替换

  运行并验证 FAILS：
  ```bash
  cd hermind-desktop/tests/widgets && qmake ai_provider_settings_widget_test.pro && make
  ```
  预期：编译失败（`ai_provider_settings_widget.h` 不存在）。
- [ ] 编写完整实现。

  `ai_provider_settings_widget.h`：
  ```cpp
  #ifndef AI_PROVIDER_SETTINGS_WIDGET_H
  #define AI_PROVIDER_SETTINGS_WIDGET_H

  #include <QHash>
  #include <QWidget>

  class HermindApiClient;
  class QLabel;
  class QStackedWidget;
  class QButtonGroup;
  class SidebarMenuButton;

  // Frame for /settings/* AI provider pages (roadmap phase 3.0).
  // Tabs: llm-preference, vector-database, embedding-preference,
  // text-splitter-preference, audio-preference, transcription-preference,
  // model-routers. Sub-phases 3.1-3.7 inject native or WebView pages
  // via setTabWidget().
  class AiProviderSettingsWidget : public QWidget
  {
      Q_OBJECT

  public:
      explicit AiProviderSettingsWidget(HermindApiClient *apiClient,
                                        QWidget *parent = nullptr);

      QString currentTabId() const;
      void setTabWidget(const QString &tabId, QWidget *widget);

  public slots:
      void setActiveTab(const QString &tabId);
      // Single-user mode (empty role) and "admin" see all tabs;
      // any other role hides them (mirrors web roles: ["admin"]).
      void setUserRole(const QString &role);

  signals:
      void returnClicked();
      void tabChanged(const QString &tabId);

  private slots:
      void onTabButtonClicked();
      void applyStyle();

  private:
      void buildUi();

      HermindApiClient *m_apiClient = nullptr;

      QStackedWidget *m_contentStack = nullptr;
      QLabel *m_headerTitleLabel = nullptr;
      QButtonGroup *m_tabGroup = nullptr;
      QHash<QString, SidebarMenuButton *> m_tabButtons;
      QString m_currentTabId;
  };

  #endif // AI_PROVIDER_SETTINGS_WIDGET_H
  ```

  `ai_provider_settings_widget.cpp`（完整）：
  ```cpp
  #include "ai_provider_settings_widget.h"
  #include "ai_provider_settings_tab.h"
  #include "sidebar_menu_button.h"
  #include "icon_button.h"
  #include "theme_colors.h"
  #include "theme_manager.h"
  #include "auth_manager.h"

  #include <QButtonGroup>
  #include <QHBoxLayout>
  #include <QLabel>
  #include <QStackedWidget>
  #include <QVBoxLayout>

  AiProviderSettingsWidget::AiProviderSettingsWidget(HermindApiClient *apiClient,
                                                     QWidget *parent)
      : QWidget(parent)
      , m_apiClient(apiClient)
  {
      buildUi();
      applyStyle();
      connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
              this, &AiProviderSettingsWidget::applyStyle);
      connect(&AuthManager::instance(), &AuthManager::userChanged,
              this, [this](const HermindUser &user) {
                  setUserRole(user.role());
              });
      setUserRole(AuthManager::instance().currentUser().role());
  }

  QString AiProviderSettingsWidget::currentTabId() const
  {
      return m_currentTabId;
  }

  void AiProviderSettingsWidget::setActiveTab(const QString &tabId)
  {
      int idx = AiProviderSettingsTabs::indexOf(tabId);
      if (idx < 0)
          idx = 0;

      const QString actualId = AiProviderSettingsTabs::all().at(idx).id;
      if (m_currentTabId == actualId && m_contentStack->currentIndex() == idx)
          return;

      m_currentTabId = actualId;
      m_contentStack->setCurrentIndex(idx);

      SidebarMenuButton *btn = m_tabButtons.value(actualId);
      if (btn && !btn->isChecked())
          btn->setChecked(true);

      m_headerTitleLabel->setText(AiProviderSettingsTabs::titleOf(actualId));

      emit tabChanged(actualId);
  }

  void AiProviderSettingsWidget::setUserRole(const QString &role)
  {
      // Single-user mode yields an empty role (AuthManager keeps an empty
      // user); web treats "no user" as full access. Any non-empty role
      // other than admin is denied (web: roles: ["admin"]).
      const bool isAdmin = role.isEmpty() || role == QStringLiteral("admin");

      for (SidebarMenuButton *btn : m_tabButtons)
          btn->setVisible(isAdmin);

      if (!isAdmin && m_currentTabId != QStringLiteral("llm-preference"))
          setActiveTab(QStringLiteral("llm-preference"));
  }

  void AiProviderSettingsWidget::setTabWidget(const QString &tabId, QWidget *widget)
  {
      if (!widget || !m_contentStack)
          return;

      const int idx = AiProviderSettingsTabs::indexOf(tabId);
      if (idx < 0)
          return;

      QWidget *old = m_contentStack->widget(idx);
      if (old) {
          m_contentStack->removeWidget(old);
          old->deleteLater();
      }

      widget->setParent(m_contentStack);
      m_contentStack->insertWidget(idx, widget);

      if (m_currentTabId == tabId)
          m_contentStack->setCurrentIndex(idx);
  }

  void AiProviderSettingsWidget::onTabButtonClicked()
  {
      auto *btn = qobject_cast<SidebarMenuButton *>(sender());
      if (!btn)
          return;
      setActiveTab(btn->property("tabId").toString());
  }

  void AiProviderSettingsWidget::applyStyle()
  {
      const bool dark = ThemeManager::instance().isDarkMode();
      const QString windowBg = ThemeColors::windowBackground(dark).name();
      const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
      const QString textPrimary = ThemeColors::textPrimary(dark).name();

      setStyleSheet(QStringLiteral(R"(
          AiProviderSettingsWidget {
              background-color: %1;
          }
          #aiProviderSidebar {
              background-color: %2;
              border: none;
          }
          #aiProviderHeaderLabel {
              color: %3;
              font-size: 16px;
              font-weight: 600;
          }
          #headerTitleLabel {
              color: %3;
              font-size: 20px;
              font-weight: 600;
          }
          #settingsContent {
              background-color: %1;
              border: none;
          }
      )").arg(windowBg, sidebarBg, textPrimary));

      m_headerTitleLabel->setStyleSheet(QStringLiteral(
          "QLabel { color: %1; font-size: 20px; font-weight: 600; }"
      ).arg(textPrimary));
  }

  void AiProviderSettingsWidget::buildUi()
  {
      auto *rootLayout = new QHBoxLayout(this);
      rootLayout->setContentsMargins(0, 0, 0, 0);
      rootLayout->setSpacing(0);

      // Sidebar
      auto *sidebar = new QWidget(this);
      sidebar->setObjectName(QStringLiteral("aiProviderSidebar"));
      sidebar->setFixedWidth(260);
      auto *sidebarLayout = new QVBoxLayout(sidebar);
      sidebarLayout->setContentsMargins(16, 16, 16, 16);
      sidebarLayout->setSpacing(12);

      auto *headerLabel = new QLabel(tr("AI Providers"), sidebar);
      headerLabel->setObjectName(QStringLiteral("aiProviderHeaderLabel"));
      headerLabel->setWordWrap(true);
      sidebarLayout->addWidget(headerLabel);

      auto *backButton = new IconButton(sidebar);
      backButton->setObjectName(QStringLiteral("returnButton"));
      backButton->setIconText(QStringLiteral("←"));
      backButton->setToolTip(tr("Return"));
      connect(backButton, &IconButton::clicked,
              this, &AiProviderSettingsWidget::returnClicked);
      sidebarLayout->addWidget(backButton);

      sidebarLayout->addSpacing(8);

      m_tabGroup = new QButtonGroup(this);
      m_tabGroup->setExclusive(true);

      const auto &tabs = AiProviderSettingsTabs::all();
      for (const AiProviderSettingsTab &tab : tabs) {
          auto *btn = new SidebarMenuButton(tab.title, sidebar);
          btn->setObjectName(QStringLiteral("tabButton_") + tab.id);
          btn->setProperty("tabId", tab.id);
          btn->setCheckable(true);
          m_tabGroup->addButton(btn);
          m_tabButtons.insert(tab.id, btn);
          connect(btn, &SidebarMenuButton::clicked,
                  this, &AiProviderSettingsWidget::onTabButtonClicked);
          sidebarLayout->addWidget(btn);
      }
      sidebarLayout->addStretch();

      rootLayout->addWidget(sidebar);

      // Content
      auto *content = new QWidget(this);
      content->setObjectName(QStringLiteral("settingsContent"));
      auto *contentLayout = new QVBoxLayout(content);
      contentLayout->setContentsMargins(24, 24, 24, 24);
      contentLayout->setSpacing(16);

      m_headerTitleLabel = new QLabel(content);
      m_headerTitleLabel->setObjectName(QStringLiteral("headerTitleLabel"));
      contentLayout->addWidget(m_headerTitleLabel);

      m_contentStack = new QStackedWidget(content);
      m_contentStack->setObjectName(QStringLiteral("contentStack"));

      for (const AiProviderSettingsTab &tab : tabs) {
          auto *page = new QWidget();
          page->setObjectName(QStringLiteral("page_") + tab.id);
          auto *pageLayout = new QVBoxLayout(page);
          pageLayout->addStretch();
          auto *placeholder = new QLabel(
              tr("%1 settings will appear here").arg(tab.title));
          placeholder->setAlignment(Qt::AlignCenter);
          pageLayout->addWidget(placeholder);
          pageLayout->addStretch();
          m_contentStack->addWidget(page);
      }

      contentLayout->addWidget(m_contentStack, 1);
      rootLayout->addWidget(content, 1);

      setActiveTab(QStringLiteral("llm-preference"));
  }
  ```
  说明：`m_apiClient` 目前仅保存备用（3.1+ 的原生页通过构造参数自行持有 client）；它是成员变量，不会触发未使用警告。
- [ ] 构建运行并验证 PASSES：
  ```bash
  cd hermind-desktop/tests/widgets && qmake ai_provider_settings_widget_test.pro && make && QT_QPA_PLATFORM=offscreen ./tst_ai_provider_settings_widget
  ```
  预期：9 个测试全部通过。
- [ ] Commit：
  ```bash
  git add hermind-desktop/widgets/ai_provider_settings_widget.h hermind-desktop/widgets/ai_provider_settings_widget.cpp hermind-desktop/tests/widgets/tst_ai_provider_settings_widget.cpp hermind-desktop/tests/widgets/ai_provider_settings_widget_test.pro
  git commit -m "feat(desktop): add AI provider settings frame widget (3.0)"
  ```

## Self-review (Part 1)

- [ ] 1. Spec-coverage: 本 Part 覆盖的 spec 项（7 Tab 二级框架、`setTabWidget` 注册接口、角色 gating、主题跟随、控件测试）全部实现；WebView 桥接与页面内容仍属 index 中 no-op 项。
- [ ] 2. Placeholder scan: 无 TODO/TBD；注册表与控件均给出完整代码；内容占位页是有意的交付物（3.1–3.7 替换）。
- [ ] 3. No phantom tasks: Task 1 产出注册表 + 5 项测试；Task 2 产出控件 + 9 项测试，均可独立验证。
- [ ] 4. Dependency soundness: Task 2 仅用 Task 1 的 `AiProviderSettingsTabs`；测试 include 的 `sidebar_menu_button.h`、`icon_button.h`、`theme_colors.h`、`theme_manager.h`、`auth_manager.h` 均为既有文件（已在 `workspace_settings_widget_test.pro` 中验证可链接）。
- [ ] 5. Caller & build soundness: 均为新增类，无共享签名变更；测试 .pro 复制自已验证的 `workspace_settings_widget_test.pro` 依赖集。
- [ ] 6. Test-the-risk: 角色 gating（状态突变）有行为测试；单机空 role must-survive 场景由 `buttonsVisibleByDefaultForSingleUser` 锁定，断言与实现常量（`role.isEmpty() || role == "admin"`）一致；7 个 Tab id 断言与注册表常量逐项一致。
- [ ] 7. Type consistency: 测试引用的 `currentTabId()`、`setTabWidget(const QString &, QWidget *)`、`setUserRole(const QString &)`、`tabChanged(const QString &)` 信号与头文件声明完全一致。
