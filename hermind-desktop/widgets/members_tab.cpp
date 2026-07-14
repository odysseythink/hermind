#include "members_tab.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QDateTime>
#include <QHBoxLayout>
#include <QHeaderView>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QTableWidget>
#include <QVBoxLayout>

// ---------------------------------------------------------------------------
// ManageMembersDialog
// ---------------------------------------------------------------------------

ManageMembersDialog::ManageMembersDialog(HermindApiClient *apiClient,
                                         int workspaceId,
                                         const QVector<int> &currentMemberIds,
                                         QWidget *parent)
    : QDialog(parent)
    , m_apiClient(apiClient)
    , m_workspaceId(workspaceId)
    , m_memberIds(currentMemberIds)
{
    setWindowTitle(tr("Manage workspace users"));
    setMinimumSize(520, 420);
    buildUi();

    if (m_apiClient && m_workspaceId > 0) {
        m_saveButton->setEnabled(false);
        m_apiClient->listUsers(
            [this](const QVector<HermindUser> &users, const ApiError &error) {
                onUsersLoaded(users, error);
            });
    }
}

void ManageMembersDialog::onUsersLoaded(const QVector<HermindUser> &users,
                                        const ApiError &error)
{
    m_saveButton->setEnabled(true);
    if (!error.isEmpty()) {
        QMessageBox::warning(this, tr("Load failed"), error.message());
        return;
    }
    m_users = users;
    populateList();
}

void ManageMembersDialog::onSearchChanged(const QString &)
{
    populateList();
}

void ManageMembersDialog::onSelectAllClicked()
{
    for (int i = 0; i < m_userList->count(); ++i)
        m_userList->item(i)->setCheckState(Qt::Checked);
}

void ManageMembersDialog::onUnselectClicked()
{
    for (int i = 0; i < m_userList->count(); ++i)
        m_userList->item(i)->setCheckState(Qt::Unchecked);
}

void ManageMembersDialog::onSaveClicked()
{
    if (!m_apiClient || m_workspaceId <= 0)
        return;

    m_saveButton->setEnabled(false);
    m_apiClient->updateWorkspaceUsers(m_workspaceId, checkedUserIds(),
        [this](bool success, const QString &message, const ApiError &error) {
            onUpdateFinished(success, message, error);
        });
}

void ManageMembersDialog::onUpdateFinished(bool success,
                                           const QString &,
                                           const ApiError &error)
{
    m_saveButton->setEnabled(true);
    if (!success || !error.isEmpty()) {
        QMessageBox::warning(this, tr("Save failed"),
                             error.isEmpty() ? tr("Please try again later")
                                             : error.message());
        return;
    }
    emit membersUpdated();
    accept();
}

void ManageMembersDialog::buildUi()
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(16, 16, 16, 16);
    layout->setSpacing(12);

    m_searchEdit = new QLineEdit(this);
    m_searchEdit->setObjectName(QStringLiteral("memberSearchEdit"));
    m_searchEdit->setPlaceholderText(tr("Search for a user"));
    connect(m_searchEdit, &QLineEdit::textChanged,
            this, &ManageMembersDialog::onSearchChanged);
    layout->addWidget(m_searchEdit);

    m_userList = new QListWidget(this);
    m_userList->setObjectName(QStringLiteral("memberUserList"));
    layout->addWidget(m_userList, 1);

    auto *footer = new QHBoxLayout();
    m_selectAllButton = new QPushButton(tr("Select All"), this);
    m_selectAllButton->setObjectName(QStringLiteral("selectAllButton"));
    connect(m_selectAllButton, &QPushButton::clicked,
            this, &ManageMembersDialog::onSelectAllClicked);
    footer->addWidget(m_selectAllButton);

    m_unselectButton = new QPushButton(tr("Unselect"), this);
    m_unselectButton->setObjectName(QStringLiteral("unselectButton"));
    connect(m_unselectButton, &QPushButton::clicked,
            this, &ManageMembersDialog::onUnselectClicked);
    footer->addWidget(m_unselectButton);

    footer->addStretch();

    m_saveButton = new QPushButton(tr("Save"), this);
    m_saveButton->setObjectName(QStringLiteral("saveMembersButton"));
    connect(m_saveButton, &QPushButton::clicked,
            this, &ManageMembersDialog::onSaveClicked);
    footer->addWidget(m_saveButton);

    layout->addLayout(footer);
}

void ManageMembersDialog::populateList()
{
    const QString filter = m_searchEdit->text().trimmed().toLower();
    m_userList->clear();

    for (const HermindUser &user : m_users) {
        if (!isSelectable(user))
            continue;
        if (!filter.isEmpty() && !user.username().toLower().contains(filter))
            continue;

        auto *item = new QListWidgetItem(user.username(), m_userList);
        item->setData(Qt::UserRole, user.id());
        item->setFlags(item->flags() | Qt::ItemIsUserCheckable);
        item->setCheckState(m_memberIds.contains(user.id())
                                ? Qt::Checked : Qt::Unchecked);
    }
}

bool ManageMembersDialog::isSelectable(const HermindUser &user) const
{
    // Admins and managers have implicit access to every workspace
    // (mirrors frontend AddMemberModal filtering).
    return user.role() != QStringLiteral("admin")
        && user.role() != QStringLiteral("manager");
}

QVector<int> ManageMembersDialog::checkedUserIds() const
{
    QVector<int> ids;
    for (int i = 0; i < m_userList->count(); ++i) {
        const QListWidgetItem *item = m_userList->item(i);
        if (item->checkState() == Qt::Checked)
            ids.append(item->data(Qt::UserRole).toInt());
    }
    return ids;
}

// ---------------------------------------------------------------------------
// MembersTab
// ---------------------------------------------------------------------------

MembersTab::MembersTab(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();

    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "MembersTab { background-color: transparent; }"
        "QTableWidget { color: %1; border: 1px solid %2; border-radius: 8px; }"
        "QHeaderView::section { color: %1; }")
        .arg(ThemeColors::textPrimary(dark).name(),
             ThemeColors::border(dark).name()));
}

void MembersTab::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_workspaceId = 0;
    m_table->setRowCount(0);
    m_manageButton->setEnabled(false);

    if (!m_apiClient || slug.isEmpty())
        return;

    m_apiClient->getWorkspace(slug,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceLoaded(workspace, message, error);
        });
}

void MembersTab::onWorkspaceLoaded(const HermindWorkspace &workspace,
                                   const QString &,
                                   const ApiError &error)
{
    if (!error.isEmpty()) {
        populateTable(QVector<HermindWorkspaceUser>());
        emit membersLoaded(0);
        return;
    }

    m_workspaceId = workspace.id();
    m_manageButton->setEnabled(m_workspaceId > 0);
    loadMembers();
}

void MembersTab::onMembersLoaded(const QVector<HermindWorkspaceUser> &users,
                                 const ApiError &error)
{
    if (!error.isEmpty()) {
        populateTable(QVector<HermindWorkspaceUser>());
        emit membersLoaded(0);
        return;
    }
    populateTable(users);
    emit membersLoaded(users.size());
}

void MembersTab::onManageClicked()
{
    if (!m_apiClient || m_workspaceId <= 0)
        return;

    QVector<int> memberIds;
    for (int row = 0; row < m_table->rowCount(); ++row) {
        const QTableWidgetItem *item = m_table->item(row, 0);
        if (item)
            memberIds.append(item->data(Qt::UserRole).toInt());
    }

    auto *dialog = new ManageMembersDialog(m_apiClient, m_workspaceId,
                                           memberIds, this);
    dialog->setObjectName(QStringLiteral("manageMembersDialog"));
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    connect(dialog, &ManageMembersDialog::membersUpdated,
            this, &MembersTab::loadMembers);
    dialog->open();
}

void MembersTab::buildUi()
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(16);

    auto *headerLayout = new QHBoxLayout();
    auto *title = new QLabel(tr("Members"), this);
    QFont titleFont = title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    title->setFont(titleFont);
    headerLayout->addWidget(title);
    headerLayout->addStretch();

    m_manageButton = new QPushButton(tr("Manage Users"), this);
    m_manageButton->setObjectName(QStringLiteral("manageMembersButton"));
    m_manageButton->setEnabled(false);
    connect(m_manageButton, &QPushButton::clicked,
            this, &MembersTab::onManageClicked);
    headerLayout->addWidget(m_manageButton);
    rootLayout->addLayout(headerLayout);

    m_table = new QTableWidget(this);
    m_table->setObjectName(QStringLiteral("membersTable"));
    m_table->setColumnCount(3);
    m_table->setHorizontalHeaderLabels(
        { tr("Username"), tr("Role"), tr("Date Added") });
    m_table->horizontalHeader()->setStretchLastSection(true);
    m_table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_table->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_table->verticalHeader()->setVisible(false);
    rootLayout->addWidget(m_table, 1);
}

void MembersTab::loadMembers()
{
    if (!m_apiClient || m_workspaceId <= 0)
        return;

    m_apiClient->listWorkspaceUsers(m_workspaceId,
        [this](const QVector<HermindWorkspaceUser> &users,
               const ApiError &error) {
            onMembersLoaded(users, error);
        });
}

void MembersTab::populateTable(const QVector<HermindWorkspaceUser> &users)
{
    m_table->setRowCount(0);

    if (users.isEmpty()) {
        m_table->setRowCount(1);
        auto *empty = new QTableWidgetItem(tr("No workspace members"));
        empty->setFlags(Qt::ItemIsEnabled);
        m_table->setItem(0, 0, empty);
        m_table->setSpan(0, 0, 1, 3);
        return;
    }

    m_table->setRowCount(users.size());
    for (int row = 0; row < users.size(); ++row) {
        const HermindWorkspaceUser &user = users.at(row);

        auto *nameItem = new QTableWidgetItem(user.username());
        nameItem->setData(Qt::UserRole, user.userId());
        m_table->setItem(row, 0, nameItem);

        QString role = user.role();
        if (!role.isEmpty())
            role[0] = role[0].toUpper();
        m_table->setItem(row, 1, new QTableWidgetItem(role));

        m_table->setItem(row, 2, new QTableWidgetItem(
            user.lastUpdatedAt().toString(Qt::ISODate)));
    }
}
