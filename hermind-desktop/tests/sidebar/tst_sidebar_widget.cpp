#include <QtTest>
#include <QSignalSpy>
#include "sidebar_widget.h"
#include "active_workspaces_widget.h"
#include "sidebar_footer_widget.h"
#include "search_input.h"
#include "search_box_widget.h"
#include "navigation_manager.h"

class TestSidebarWidget : public QObject
{
    Q_OBJECT
private slots:
    void initTestCase();
    void hasRequiredChildren();
    void settingsSignalPropagated();
    void searchInputHasPlaceholder();
    void searchResultSelection_navigatesToWorkspaceChat();
};

void TestSidebarWidget::initTestCase()
{
    qRegisterMetaType<NavigationRoute>("NavigationRoute");
}

void TestSidebarWidget::hasRequiredChildren()
{
    SidebarWidget sidebar;
    QVERIFY(!sidebar.findChildren<SearchInput *>().isEmpty());
    QVERIFY(!sidebar.findChildren<ActiveWorkspacesWidget *>().isEmpty());
    QVERIFY(!sidebar.findChildren<SidebarFooterWidget *>().isEmpty());
}

void TestSidebarWidget::settingsSignalPropagated()
{
    SidebarWidget sidebar;
    QSignalSpy spy(&sidebar, &SidebarWidget::openSettingsRequested);
    sidebar.clickSettingsButton();
    QCOMPARE(spy.count(), 1);
}

void TestSidebarWidget::searchInputHasPlaceholder()
{
    SidebarWidget sidebar;
    SearchInput *input = sidebar.findChild<SearchInput *>();
    QVERIFY(input);
    QCOMPARE(input->placeholderText(), QStringLiteral("搜索"));
}

void TestSidebarWidget::searchResultSelection_navigatesToWorkspaceChat()
{
    SidebarWidget sidebar;
    SearchBoxWidget *searchBox = sidebar.findChild<SearchBoxWidget *>();
    QVERIFY(searchBox);

    QSignalSpy routeSpy(&NavigationManager::instance(), &NavigationManager::currentRouteChanged);
    searchBox->resultSelected(QStringLiteral("ws-1"), QStringLiteral("t-1"));

    QCOMPARE(routeSpy.count(), 1);
    const NavigationRoute route = routeSpy.at(0).at(0).value<NavigationRoute>();
    QVERIFY(route.page == NavigationPage::WorkspaceChat);
    QCOMPARE(route.workspaceSlug, QStringLiteral("ws-1"));
    QCOMPARE(route.threadSlug, QStringLiteral("t-1"));
}

QTEST_MAIN(TestSidebarWidget)
#include "tst_sidebar_widget.moc"
