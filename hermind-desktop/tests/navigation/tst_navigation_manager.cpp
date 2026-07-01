#include <QtTest>
#include <QSignalSpy>
#include "navigation_manager.h"

class TestNavigationManager : public QObject
{
    Q_OBJECT

private slots:
    void init();

    void singletonReturnsSameInstance();
    void initialRouteIsDefaultChat();
    void canGoBackInitiallyFalse();

    void navigateToAppendsRoute();
    void navigateToEmitsCurrentRouteChanged();
    void navigateToEmitsCanGoBackChanged();
    void navigateToTrimsFutureHistory();
    void goBackDecrementsIndex();
    void goBackDoesNothingAtRoot();
    void replaceWithReplacesCurrent();
    void replaceWithDoesNotChangeCanGoBack();
    void clearHistoryResetsToDefaultChat();
    void historyDepthLimit();
};

void TestNavigationManager::init()
{
    NavigationManager::instance().clearHistory();
}

void TestNavigationManager::singletonReturnsSameInstance()
{
    NavigationManager &a = NavigationManager::instance();
    NavigationManager &b = NavigationManager::instance();
    QCOMPARE(&a, &b);
}

void TestNavigationManager::initialRouteIsDefaultChat()
{
    NavigationManager &mgr = NavigationManager::instance();
    QCOMPARE(mgr.currentPage(), NavigationPage::DefaultChat);
    QCOMPARE(mgr.history().size(), 1);
    QVERIFY(!mgr.canGoBack());
}

void TestNavigationManager::canGoBackInitiallyFalse()
{
    QVERIFY(!NavigationManager::instance().canGoBack());
}

void TestNavigationManager::navigateToAppendsRoute()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings, QString(), QString(), "appearance", QString()});

    QCOMPARE(mgr.history().size(), 2);
    QCOMPARE(mgr.currentPage(), NavigationPage::GeneralSettings);
    QCOMPARE(mgr.currentRoute().settingsPath, QStringLiteral("appearance"));
    QVERIFY(mgr.canGoBack());
}

void TestNavigationManager::navigateToEmitsCurrentRouteChanged()
{
    NavigationManager &mgr = NavigationManager::instance();
    QSignalSpy spy(&mgr, &NavigationManager::currentRouteChanged);

    mgr.navigateTo({NavigationPage::WorkspaceChat, "ws-slug", "t-1", QString(), QString()});

    QCOMPARE(spy.count(), 1);
    const NavigationRoute emitted = spy.at(0).at(0).value<NavigationRoute>();
    QCOMPARE(emitted.page, NavigationPage::WorkspaceChat);
    QCOMPARE(emitted.workspaceSlug, QStringLiteral("ws-slug"));
    QCOMPARE(emitted.threadSlug, QStringLiteral("t-1"));
}

void TestNavigationManager::navigateToEmitsCanGoBackChanged()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.clearHistory();
    QVERIFY(!mgr.canGoBack());

    QSignalSpy spy(&mgr, &NavigationManager::canGoBackChanged);
    mgr.navigateTo({NavigationPage::GeneralSettings});

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).toBool(), true);
}

void TestNavigationManager::navigateToTrimsFutureHistory()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings});
    mgr.navigateTo({NavigationPage::AdminSettings, QString(), QString(), "users", QString()});
    mgr.goBack(); // now at GeneralSettings

    mgr.navigateTo({NavigationPage::WorkspaceSettings, "ws", QString(), "members", QString()});

    QCOMPARE(mgr.history().size(), 3); // DefaultChat, GeneralSettings, WorkspaceSettings
    QCOMPARE(mgr.currentPage(), NavigationPage::WorkspaceSettings);
}

void TestNavigationManager::goBackDecrementsIndex()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings});
    mgr.navigateTo({NavigationPage::AdminSettings, QString(), QString(), "users", QString()});

    QSignalSpy routeSpy(&mgr, &NavigationManager::currentRouteChanged);
    QSignalSpy backSpy(&mgr, &NavigationManager::canGoBackChanged);

    mgr.goBack();

    QCOMPARE(mgr.currentPage(), NavigationPage::GeneralSettings);
    QCOMPARE(routeSpy.count(), 1);
    QCOMPARE(backSpy.count(), 0); // still can go back to DefaultChat
    QVERIFY(mgr.canGoBack());

    mgr.goBack();
    QCOMPARE(mgr.currentPage(), NavigationPage::DefaultChat);
    QVERIFY(!mgr.canGoBack());
    QCOMPARE(backSpy.count(), 1);
}

void TestNavigationManager::goBackDoesNothingAtRoot()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.clearHistory();
    QVERIFY(!mgr.canGoBack());

    QSignalSpy routeSpy(&mgr, &NavigationManager::currentRouteChanged);
    mgr.goBack();

    QCOMPARE(routeSpy.count(), 0);
    QCOMPARE(mgr.currentPage(), NavigationPage::DefaultChat);
}

void TestNavigationManager::replaceWithReplacesCurrent()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings, QString(), QString(), "appearance", QString()});

    QSignalSpy routeSpy(&mgr, &NavigationManager::currentRouteChanged);
    QSignalSpy historySpy(&mgr, &NavigationManager::historyChanged);

    mgr.replaceWith({NavigationPage::GeneralSettings, QString(), QString(), "security", QString()});

    QCOMPARE(mgr.history().size(), 2);
    QCOMPARE(mgr.currentRoute().settingsPath, QStringLiteral("security"));
    QCOMPARE(routeSpy.count(), 1);
    QCOMPARE(historySpy.count(), 1);
}

void TestNavigationManager::replaceWithDoesNotChangeCanGoBack()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings});

    QSignalSpy backSpy(&mgr, &NavigationManager::canGoBackChanged);
    mgr.replaceWith({NavigationPage::AdminSettings, QString(), QString(), "users", QString()});

    QCOMPARE(backSpy.count(), 0);
    QVERIFY(mgr.canGoBack());
}

void TestNavigationManager::clearHistoryResetsToDefaultChat()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.navigateTo({NavigationPage::GeneralSettings});
    mgr.navigateTo({NavigationPage::AdminSettings, QString(), QString(), "users", QString()});

    QSignalSpy historySpy(&mgr, &NavigationManager::historyChanged);
    QSignalSpy backSpy(&mgr, &NavigationManager::canGoBackChanged);

    mgr.clearHistory();

    QCOMPARE(mgr.history().size(), 1);
    QCOMPARE(mgr.currentPage(), NavigationPage::DefaultChat);
    QVERIFY(!mgr.canGoBack());
    QCOMPARE(historySpy.count(), 1);
    QCOMPARE(backSpy.count(), 1);
}

void TestNavigationManager::historyDepthLimit()
{
    NavigationManager &mgr = NavigationManager::instance();
    mgr.clearHistory();

    for (int i = 0; i < 55; ++i) {
        mgr.navigateTo({NavigationPage::GeneralSettings, QString(), QString(), QString::number(i), QString()});
    }

    // 51 current + 50 past = max 51 items; exact limit is implementation detail,
    // but must not exceed kMaxHistoryDepth + 1.
    QVERIFY(mgr.history().size() <= 51);
    QVERIFY(mgr.canGoBack());
}

QTEST_MAIN(TestNavigationManager)
#include "tst_navigation_manager.moc"
