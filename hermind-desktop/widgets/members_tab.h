#ifndef MEMBERS_TAB_H
#define MEMBERS_TAB_H

#include <QDialog>
#include <QWidget>

#include "api_response.h"
#include "hermind_user.h"
#include "hermind_workspace.h"
#include "hermind_workspace_user.h"

class HermindApiClient;
class QLineEdit;
class QListWidget;
class QPushButton;
class QTableWidget;

/// Dialog for managing which users belong to a workspace.
/// Lists all non-admin/non-manager users as checkable items, pre-checked
/// with the current members. Saving replaces the workspace member set
/// (mirrors frontend AddMemberModal).
class ManageMembersDialog : public QDialog
{
    Q_OBJECT

public:
    ManageMembersDialog(HermindApiClient *apiClient,
                        int workspaceId,
                        const QVector<int> &currentMemberIds,
                        QWidget *parent = nullptr);

signals:
    void membersUpdated();

private slots:
    void onUsersLoaded(const QVector<HermindUser> &users,
                       const ApiError &error);
    void onSearchChanged(const QString &text);
    void onSelectAllClicked();
    void onUnselectClicked();
    void onSaveClicked();
    void onUpdateFinished(bool success,
                          const QString &message,
                          const ApiError &error);

private:
    void buildUi();
    void populateList();
    bool isSelectable(const HermindUser &user) const;
    QVector<int> checkedUserIds() const;

    HermindApiClient *m_apiClient = nullptr;
    int m_workspaceId = 0;
    QVector<int> m_memberIds;
    QVector<HermindUser> m_users;

    QLineEdit *m_searchEdit = nullptr;
    QListWidget *m_userList = nullptr;
    QPushButton *m_selectAllButton = nullptr;
    QPushButton *m_unselectButton = nullptr;
    QPushButton *m_saveButton = nullptr;
};

/// Workspace settings tab: lists current workspace members and opens
/// ManageMembersDialog to add/remove members (mirrors frontend Members page).
class MembersTab : public QWidget
{
    Q_OBJECT

public:
    explicit MembersTab(HermindApiClient *apiClient,
                        QWidget *parent = nullptr);

    void setWorkspaceSlug(const QString &slug);

signals:
    void membersLoaded(int count);

private slots:
    void onWorkspaceLoaded(const HermindWorkspace &workspace,
                           const QString &message,
                           const ApiError &error);
    void onMembersLoaded(const QVector<HermindWorkspaceUser> &users,
                         const ApiError &error);
    void onManageClicked();

private:
    void buildUi();
    void loadMembers();
    void populateTable(const QVector<HermindWorkspaceUser> &users);

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    int m_workspaceId = 0;

    QTableWidget *m_table = nullptr;
    QPushButton *m_manageButton = nullptr;
};

#endif // MEMBERS_TAB_H
