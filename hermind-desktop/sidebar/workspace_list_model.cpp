#include "workspace_list_model.h"
#include "hermind_workspace.h"

WorkspaceListModel::WorkspaceListModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

int WorkspaceListModel::rowCount(const QModelIndex &parent) const
{
    if (parent.isValid())
        return 0;
    return m_workspaces.size();
}

QVariant WorkspaceListModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= m_workspaces.size())
        return QVariant();

    const HermindWorkspace &ws = m_workspaces.at(index.row());
    switch (role) {
    case Qt::DisplayRole:
    case NameRole:
        return ws.name();
    case SlugRole:
        return ws.slug();
    case IdRole:
        return ws.id();
    case SelectedRole:
        return ws.slug() == m_selectedSlug;
    default:
        return QVariant();
    }
}

QHash<int, QByteArray> WorkspaceListModel::roleNames() const
{
    QHash<int, QByteArray> roles;
    roles.insert(NameRole, QByteArrayLiteral("name"));
    roles.insert(SlugRole, QByteArrayLiteral("slug"));
    roles.insert(IdRole, QByteArrayLiteral("id"));
    roles.insert(SelectedRole, QByteArrayLiteral("selected"));
    return roles;
}

void WorkspaceListModel::setWorkspaces(const QVector<HermindWorkspace> &workspaces)
{
    beginResetModel();
    m_workspaces = workspaces;
    endResetModel();
}

const QVector<HermindWorkspace> &WorkspaceListModel::workspaces() const
{
    return m_workspaces;
}

void WorkspaceListModel::setSelectedSlug(const QString &slug)
{
    if (m_selectedSlug == slug)
        return;
    m_selectedSlug = slug;
    emit dataChanged(index(0, 0), index(rowCount() - 1, 0), {SelectedRole});
}

QString WorkspaceListModel::selectedSlug() const
{
    return m_selectedSlug;
}
