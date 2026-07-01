#include <QtTest>
#include "workspace_list_model.h"
#include "hermind_workspace.h"

class TestWorkspaceListModel : public QObject
{
    Q_OBJECT
private slots:
    void emptyModelHasZeroRows();
    void setWorkspacesPopulatesModel();
    void selectedSlugRoleReflectsSelection();
    void roleNamesContainsSlugRole();
};

void TestWorkspaceListModel::emptyModelHasZeroRows()
{
    WorkspaceListModel model;
    QCOMPARE(model.rowCount(QModelIndex()), 0);
}

void TestWorkspaceListModel::setWorkspacesPopulatesModel()
{
    WorkspaceListModel model;
    QVector<HermindWorkspace> list;
    HermindWorkspace ws;
    ws = HermindWorkspace::fromJson(QJsonObject{
        {"id", 1}, {"name", "Default"}, {"slug", "default"}, {"openAiHistory", 20}
    });
    list.append(ws);
    ws = HermindWorkspace::fromJson(QJsonObject{
        {"id", 2}, {"name", "KB"}, {"slug", "kb"}, {"openAiHistory", 20}
    });
    list.append(ws);

    model.setWorkspaces(list);
    QCOMPARE(model.rowCount(QModelIndex()), 2);
    QCOMPARE(model.data(model.index(0, 0), WorkspaceListModel::NameRole).toString(),
             QStringLiteral("Default"));
    QCOMPARE(model.data(model.index(1, 0), WorkspaceListModel::SlugRole).toString(),
             QStringLiteral("kb"));
}

void TestWorkspaceListModel::selectedSlugRoleReflectsSelection()
{
    WorkspaceListModel model;
    QVector<HermindWorkspace> list;
    list.append(HermindWorkspace::fromJson(QJsonObject{
        {"id", 1}, {"name", "A"}, {"slug", "a"}, {"openAiHistory", 20}
    }));
    list.append(HermindWorkspace::fromJson(QJsonObject{
        {"id", 2}, {"name", "B"}, {"slug", "b"}, {"openAiHistory", 20}
    }));
    model.setWorkspaces(list);

    QVERIFY(!model.data(model.index(0, 0), WorkspaceListModel::SelectedRole).toBool());
    model.setSelectedSlug(QStringLiteral("a"));
    QVERIFY(model.data(model.index(0, 0), WorkspaceListModel::SelectedRole).toBool());
    QVERIFY(!model.data(model.index(1, 0), WorkspaceListModel::SelectedRole).toBool());
}

void TestWorkspaceListModel::roleNamesContainsSlugRole()
{
    WorkspaceListModel model;
    const QHash<int, QByteArray> roles = model.roleNames();
    QVERIFY(roles.contains(WorkspaceListModel::SlugRole));
    QCOMPARE(roles.value(WorkspaceListModel::SlugRole), QByteArrayLiteral("slug"));
}

QTEST_MAIN(TestWorkspaceListModel)
#include "tst_workspace_list_model.moc"
