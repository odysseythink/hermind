#include <QtTest>
#include "navigation_route.h"

class TestNavigationRoute : public QObject
{
    Q_OBJECT

private slots:
    void defaultRouteIsDefaultChat();
    void routeEqualityComparesAllFields();
    void routeInequalityWorks();
};

void TestNavigationRoute::defaultRouteIsDefaultChat()
{
    NavigationRoute r;
    QCOMPARE(r.page, NavigationPage::DefaultChat);
    QVERIFY(r.workspaceSlug.isEmpty());
    QVERIFY(r.threadSlug.isEmpty());
    QVERIFY(r.settingsPath.isEmpty());
    QVERIFY(r.inviteCode.isEmpty());
}

void TestNavigationRoute::routeEqualityComparesAllFields()
{
    NavigationRoute a{NavigationPage::WorkspaceChat, "my-ws", "thread-1", QString(), QString()};
    NavigationRoute b{NavigationPage::WorkspaceChat, "my-ws", "thread-1", QString(), QString()};
    QVERIFY(a == b);

    NavigationRoute c{NavigationPage::WorkspaceChat, "my-ws", "thread-2", QString(), QString()};
    QVERIFY(a != c);

    NavigationRoute d{NavigationPage::WorkspaceSettings, "my-ws", QString(), "general-appearance", QString()};
    QVERIFY(a != d);
}

void TestNavigationRoute::routeInequalityWorks()
{
    NavigationRoute a{NavigationPage::GeneralSettings, QString(), QString(), "llm-preference", QString()};
    NavigationRoute b{NavigationPage::GeneralSettings, QString(), QString(), "llm-preference", QString()};
    QVERIFY(!(a != b));
}

QTEST_MAIN(TestNavigationRoute)
#include "tst_navigation_route.moc"
