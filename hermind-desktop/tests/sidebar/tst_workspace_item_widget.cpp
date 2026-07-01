#include <QtTest>
#include <QSignalSpy>
#include "workspace_item_widget.h"
#include "hermind_workspace.h"

class TestWorkspaceItemWidget : public QObject
{
    Q_OBJECT
private slots:
    void displaysWorkspaceName();
    void clickEmitsWorkspaceClicked();
    void activeStateStyle();
};

void TestWorkspaceItemWidget::displaysWorkspaceName()
{
    WorkspaceItemWidget item;
    const HermindWorkspace ws = HermindWorkspace::fromJson(QJsonObject{
        {"id", 1}, {"name", "My Workspace"}, {"slug", "my-ws"}, {"openAiHistory", 20}
    });
    item.setWorkspace(ws);
    QCOMPARE(item.workspaceSlug(), QStringLiteral("my-ws"));
}

void TestWorkspaceItemWidget::clickEmitsWorkspaceClicked()
{
    WorkspaceItemWidget item;
    const HermindWorkspace ws = HermindWorkspace::fromJson(QJsonObject{
        {"id", 1}, {"name", "My Workspace"}, {"slug", "my-ws"}, {"openAiHistory", 20}
    });
    item.setWorkspace(ws);

    QSignalSpy spy(&item, &WorkspaceItemWidget::workspaceClicked);
    item.simulateClick();
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.takeFirst().at(0).toString(), QStringLiteral("my-ws"));
}

void TestWorkspaceItemWidget::activeStateStyle()
{
    WorkspaceItemWidget item;
    const HermindWorkspace ws = HermindWorkspace::fromJson(QJsonObject{
        {"id", 1}, {"name", "My Workspace"}, {"slug", "my-ws"}, {"openAiHistory", 20}
    });
    item.setWorkspace(ws);
    QVERIFY(!item.isActive());
    item.setActive(true);
    QVERIFY(item.isActive());
}

QTEST_MAIN(TestWorkspaceItemWidget)
#include "tst_workspace_item_widget.moc"
