#ifndef WORKSPACE_LIST_MODEL_H
#define WORKSPACE_LIST_MODEL_H

#include <QAbstractListModel>
#include <QVector>

#include "hermind_workspace.h"

class WorkspaceListModel : public QAbstractListModel
{
    Q_OBJECT

public:
    enum Roles {
        NameRole = Qt::UserRole + 1,
        SlugRole,
        IdRole,
        SelectedRole
    };
    Q_ENUM(Roles)

    explicit WorkspaceListModel(QObject *parent = nullptr);

    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role = Qt::DisplayRole) const override;
    QHash<int, QByteArray> roleNames() const override;

    void setWorkspaces(const QVector<HermindWorkspace> &workspaces);
    const QVector<HermindWorkspace> &workspaces() const;

    void setSelectedSlug(const QString &slug);
    QString selectedSlug() const;

private:
    QVector<HermindWorkspace> m_workspaces;
    QString m_selectedSlug;
};

#endif // WORKSPACE_LIST_MODEL_H
