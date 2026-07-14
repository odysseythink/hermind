#include <QtTest/QtTest>

#include "workspace_settings_widget.h"
#include "sidebar_menu_button.h"

class TestWorkspaceSettingsWidget : public QObject
{
    Q_OBJECT

private slots:
    void membersButtonHiddenForDefaultUser();
    void membersButtonVisibleForAdmin();
    void membersButtonVisibleForManager();
    void membersButtonHiddenForMemberRole();
    void activeMembersTabFallsBackWhenRoleRevoked();
};

void TestWorkspaceSettingsWidget::membersButtonHiddenForDefaultUser()
{
    WorkspaceSettingsWidget w(nullptr);
    auto *btn = w.findChild<SidebarMenuButton *>(QStringLiteral("tabButton_members"));
    QVERIFY(btn);
    QVERIFY(btn->isHidden());
}

void TestWorkspaceSettingsWidget::membersButtonVisibleForAdmin()
{
    WorkspaceSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("admin"));
    auto *btn = w.findChild<SidebarMenuButton *>(QStringLiteral("tabButton_members"));
    QVERIFY(btn);
    QVERIFY(!btn->isHidden());
}

void TestWorkspaceSettingsWidget::membersButtonVisibleForManager()
{
    WorkspaceSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("manager"));
    auto *btn = w.findChild<SidebarMenuButton *>(QStringLiteral("tabButton_members"));
    QVERIFY(btn);
    QVERIFY(!btn->isHidden());
}

void TestWorkspaceSettingsWidget::membersButtonHiddenForMemberRole()
{
    WorkspaceSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("admin"));
    w.setUserRole(QStringLiteral("member"));
    auto *btn = w.findChild<SidebarMenuButton *>(QStringLiteral("tabButton_members"));
    QVERIFY(btn);
    QVERIFY(btn->isHidden());
}

void TestWorkspaceSettingsWidget::activeMembersTabFallsBackWhenRoleRevoked()
{
    WorkspaceSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("admin"));
    w.setActiveTab(QStringLiteral("members"));
    QCOMPARE(w.currentTabId(), QStringLiteral("members"));

    w.setUserRole(QStringLiteral("member"));
    QCOMPARE(w.currentTabId(), QStringLiteral("general-appearance"));
}

QTEST_MAIN(TestWorkspaceSettingsWidget)
#include "tst_workspace_settings_widget.moc"
