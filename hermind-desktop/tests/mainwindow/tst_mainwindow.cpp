#include <QtTest>
#include <QStackedWidget>
#include <QToolButton>
#include <QLineEdit>
#include <QLabel>
#include <QComboBox>
#include <QDoubleSpinBox>
#include <QSpinBox>

#include "mainwindow.h"
#include "navigation_manager.h"
#include "workspace_settings_widget.h"
#include "workspace_settings_tab.h"
#include "general_appearance_tab.h"
#include "chat_settings_tab.h"
#include "vector_database_tab.h"

class TestMainWindow : public QObject
{
    Q_OBJECT

private slots:
    void init();
    void cleanup();

    void generalSettingsPageIsReachable();
    void workspaceChatNavigationFromSettingsSwitchesToChatPage();
    void workspaceSettingsPageIsReachable();
    void workspaceSettingsPageRestoresTabFromRoute();
    void workspaceSettingsReturnButtonGoesBack();
    void generalAppearanceTabIsRegisteredForGeneralAppearanceRoute();
    void chatSettingsTabIsRegisteredForChatSettingsRoute();
    void vectorDatabaseTabIsRegisteredForVectorDatabaseRoute();
};

void TestMainWindow::init()
{
    NavigationManager::instance().clearHistory();
}

void TestMainWindow::cleanup()
{
    NavigationManager::instance().clearHistory();
}

void TestMainWindow::generalSettingsPageIsReachable()
{
    MainWindow w;
    auto *stack = w.findChild<QStackedWidget *>();
    QVERIFY(stack);

    NavigationRoute route;
    route.page = NavigationPage::GeneralSettings;
    NavigationManager::instance().navigateTo(route);

    QCOMPARE(stack->currentIndex(), 1);
}

// Regression: navigating to a workspace chat while the settings page is
// showing must switch the stacked widget back to the chat page. Previously
// WorkspaceChat was never registered in MainWindow's page registry, so the
// navigation was silently dropped.
void TestMainWindow::workspaceChatNavigationFromSettingsSwitchesToChatPage()
{
    MainWindow w;
    auto *stack = w.findChild<QStackedWidget *>();
    QVERIFY(stack);

    NavigationRoute settingsRoute;
    settingsRoute.page = NavigationPage::GeneralSettings;
    NavigationManager::instance().navigateTo(settingsRoute);
    QCOMPARE(stack->currentIndex(), 1);

    NavigationRoute chatRoute;
    chatRoute.page = NavigationPage::WorkspaceChat;
    chatRoute.workspaceSlug = QStringLiteral("ws");
    NavigationManager::instance().navigateTo(chatRoute);

    QCOMPARE(stack->currentIndex(), 0);
}

void TestMainWindow::workspaceSettingsPageIsReachable()
{
    MainWindow w;
    auto *stack = w.findChild<QStackedWidget *>();
    QVERIFY(stack);

    NavigationRoute route;
    route.page = NavigationPage::WorkspaceSettings;
    route.workspaceSlug = QStringLiteral("acme");
    route.settingsPath = QStringLiteral("general-appearance");
    NavigationManager::instance().navigateTo(route);

    // MainChatWidget=0, MainSettingWidget=1, WorkspaceSettingsWidget=2
    QCOMPARE(stack->currentIndex(), 2);
}

void TestMainWindow::workspaceSettingsPageRestoresTabFromRoute()
{
    MainWindow w;

    NavigationRoute route;
    route.page = NavigationPage::WorkspaceSettings;
    route.workspaceSlug = QStringLiteral("acme");
    route.settingsPath = QStringLiteral("members");
    NavigationManager::instance().navigateTo(route);

    auto *settingsWidget = w.findChild<WorkspaceSettingsWidget *>();
    QVERIFY(settingsWidget);
    QCOMPARE(settingsWidget->currentTabId(), QStringLiteral("members"));

    auto *stack = settingsWidget->findChild<QStackedWidget *>(QStringLiteral("contentStack"));
    QVERIFY(stack);
    QCOMPARE(stack->currentIndex(), WorkspaceSettingsTabs::indexOf(QStringLiteral("members")));
}

void TestMainWindow::workspaceSettingsReturnButtonGoesBack()
{
    MainWindow w;

    NavigationRoute chatRoute;
    chatRoute.page = NavigationPage::WorkspaceChat;
    chatRoute.workspaceSlug = QStringLiteral("acme");
    NavigationManager::instance().navigateTo(chatRoute);

    NavigationRoute settingsRoute;
    settingsRoute.page = NavigationPage::WorkspaceSettings;
    settingsRoute.workspaceSlug = QStringLiteral("acme");
    settingsRoute.settingsPath = QStringLiteral("chat");
    NavigationManager::instance().navigateTo(settingsRoute);

    QVERIFY(NavigationManager::instance().canGoBack());

    auto *settingsWidget = w.findChild<WorkspaceSettingsWidget *>();
    QVERIFY(settingsWidget);
    auto *returnButton = settingsWidget->findChild<QToolButton *>(
        QStringLiteral("returnButton"));
    QVERIFY(returnButton);

    QSignalSpy spy(&NavigationManager::instance(),
                   &NavigationManager::currentRouteChanged);
    QTest::mouseClick(returnButton, Qt::LeftButton);

    QCOMPARE(NavigationManager::instance().currentPage(), NavigationPage::WorkspaceChat);
    QVERIFY(spy.count() >= 1);
}

void TestMainWindow::generalAppearanceTabIsRegisteredForGeneralAppearanceRoute()
{
    MainWindow w;

    NavigationRoute route;
    route.page = NavigationPage::WorkspaceSettings;
    route.workspaceSlug = QStringLiteral("acme");
    route.settingsPath = QStringLiteral("general-appearance");
    NavigationManager::instance().navigateTo(route);

    auto *settingsWidget = w.findChild<WorkspaceSettingsWidget *>();
    QVERIFY(settingsWidget);
    QCOMPARE(settingsWidget->currentTabId(), QStringLiteral("general-appearance"));

    auto *generalTab = settingsWidget->findChild<GeneralAppearanceTab *>();
    QVERIFY(generalTab);

    auto *nameEdit = generalTab->findChild<QLineEdit *>(QStringLiteral("workspaceNameEdit"));
    QVERIFY(nameEdit);
}

void TestMainWindow::chatSettingsTabIsRegisteredForChatSettingsRoute()
{
    MainWindow w;

    NavigationRoute route;
    route.page = NavigationPage::WorkspaceSettings;
    route.workspaceSlug = QStringLiteral("acme");
    route.settingsPath = QStringLiteral("chat");
    NavigationManager::instance().navigateTo(route);

    auto *settingsWidget = w.findChild<WorkspaceSettingsWidget *>();
    QVERIFY(settingsWidget);
    QCOMPARE(settingsWidget->currentTabId(), QStringLiteral("chat"));

    auto *chatTab = settingsWidget->findChild<ChatSettingsTab *>();
    QVERIFY(chatTab);

    auto *providerCombo = chatTab->findChild<QComboBox *>(QStringLiteral("providerCombo"));
    QVERIFY(providerCombo);
    auto *temperatureSpin = chatTab->findChild<QDoubleSpinBox *>(QStringLiteral("temperatureSpin"));
    QVERIFY(temperatureSpin);
}

void TestMainWindow::vectorDatabaseTabIsRegisteredForVectorDatabaseRoute()
{
    MainWindow w;

    NavigationRoute route;
    route.page = NavigationPage::WorkspaceSettings;
    route.workspaceSlug = QStringLiteral("acme");
    route.settingsPath = QStringLiteral("vector-database");
    NavigationManager::instance().navigateTo(route);

    auto *settingsWidget = w.findChild<WorkspaceSettingsWidget *>();
    QVERIFY(settingsWidget);
    QCOMPARE(settingsWidget->currentTabId(), QStringLiteral("vector-database"));

    auto *vectorTab = settingsWidget->findChild<VectorDatabaseTab *>();
    QVERIFY(vectorTab);

    auto *identifier = vectorTab->findChild<QLabel *>(QStringLiteral("vectorDbIdentifier"));
    QVERIFY(identifier);
    auto *topNSpin = vectorTab->findChild<QSpinBox *>(QStringLiteral("topNSpin"));
    QVERIFY(topNSpin);
    auto *thresholdCombo = vectorTab->findChild<QComboBox *>(QStringLiteral("thresholdCombo"));
    QVERIFY(thresholdCombo);
}

QTEST_MAIN(TestMainWindow)
#include "tst_mainwindow.moc"
