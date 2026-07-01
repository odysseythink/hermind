#include <QtTest>
#include <QSignalSpy>
#include "sidebar_widget.h"
#include "active_workspaces_widget.h"
#include "sidebar_footer_widget.h"
#include "search_input.h"

class TestSidebarWidget : public QObject
{
    Q_OBJECT
private slots:
    void hasRequiredChildren();
    void settingsSignalPropagated();
    void searchInputHasPlaceholder();
};

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

QTEST_MAIN(TestSidebarWidget)
#include "tst_sidebar_widget.moc"
