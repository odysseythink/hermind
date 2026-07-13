#include <QtTest>
#include <QStackedWidget>

#include "mainwindow.h"
#include "navigation_manager.h"

class TestMainWindow : public QObject
{
    Q_OBJECT

private slots:
    void init();
    void cleanup();

    void generalSettingsPageIsReachable();
    void workspaceChatNavigationFromSettingsSwitchesToChatPage();
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

QTEST_MAIN(TestMainWindow)
#include "tst_mainwindow.moc"
