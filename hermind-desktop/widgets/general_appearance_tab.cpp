#include "general_appearance_tab.h"
#include "suggested_messages_editor.h"
#include "setting_row.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPushButton>
#include <QVBoxLayout>

GeneralAppearanceTab::GeneralAppearanceTab(HermindApiClient *apiClient,
                                           QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &GeneralAppearanceTab::applyStyle);
}

void GeneralAppearanceTab::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_workspace = HermindWorkspace();
    m_nameEdit->clear();
    m_suggestedEditor->setMessages(QStringList());
    m_updateNameButton->setEnabled(false);
    m_deleteButton->setEnabled(false);

    if (!m_apiClient || slug.isEmpty())
        return;

    setLoading(true);
    m_apiClient->getWorkspace(slug,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceLoaded(workspace, message, error);
        });
}

void GeneralAppearanceTab::onWorkspaceLoaded(const HermindWorkspace &workspace,
                                             const QString &,
                                             const ApiError &error)
{
    setLoading(false);
    if (!error.isEmpty()) {
        m_nameEdit->setPlaceholderText(tr("Load failed"));
        return;
    }

    m_workspace = workspace;
    m_nameEdit->setText(workspace.name());
    m_updateNameButton->setEnabled(true);
    m_deleteButton->setEnabled(true);

    m_apiClient->getSuggestedMessages(m_workspaceSlug,
        [this](const QStringList &messages, const ApiError &error) {
            onSuggestedMessagesLoaded(messages, error);
        });
}

void GeneralAppearanceTab::onSuggestedMessagesLoaded(const QStringList &messages,
                                                     const ApiError &error)
{
    if (!error.isEmpty()) {
        m_suggestedEditor->setMessages(QStringList());
        return;
    }
    m_suggestedEditor->setMessages(messages);
}

void GeneralAppearanceTab::onWorkspaceUpdated(const HermindWorkspace &workspace,
                                              const QString &,
                                              const ApiError &error)
{
    m_updateNameButton->setEnabled(true);
    if (!error.isEmpty()) {
        QMessageBox::warning(this, tr("Update failed"), error.message());
        return;
    }
    m_workspace = workspace;
    emit workspaceUpdated(workspace);
}

void GeneralAppearanceTab::onDeleteFinished(bool success, const ApiError &error)
{
    m_deleteButton->setEnabled(true);
    if (!success || !error.isEmpty()) {
        QMessageBox::warning(this, tr("Delete failed"),
                             error.isEmpty() ? tr("Please try again later") : error.message());
        return;
    }
    emit workspaceDeleted();
}

void GeneralAppearanceTab::onUpdateNameClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    const QString newName = m_nameEdit->text().trimmed();
    if (newName.isEmpty() || newName == m_workspace.name())
        return;

    m_updateNameButton->setEnabled(false);
    QJsonObject body;
    body.insert(QStringLiteral("name"), newName);
    m_apiClient->updateWorkspace(m_workspaceSlug, body,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceUpdated(workspace, message, error);
        });
}

void GeneralAppearanceTab::onSaveSuggestionsClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    const QStringList valid = m_suggestedEditor->validMessages();
    m_apiClient->setSuggestedMessages(m_workspaceSlug, valid,
        [this](bool success, const QString &, const ApiError &error) {
            if (!success || !error.isEmpty()) {
                QMessageBox::warning(this, tr("Save failed"),
                                     error.isEmpty() ? tr("Please try again later") : error.message());
                return;
            }
            m_suggestedEditor->markSaved();
        });
}

void GeneralAppearanceTab::onDeleteClicked()
{
    if (m_workspaceSlug.isEmpty() || !m_apiClient)
        return;

    const int ret = QMessageBox::question(
        this,
        tr("Delete workspace"),
        tr("Are you sure you want to delete workspace %1? This action cannot be undone.").arg(m_workspace.name()),
        QMessageBox::Yes | QMessageBox::No,
        QMessageBox::No);

    if (ret != QMessageBox::Yes)
        return;

    m_deleteButton->setEnabled(false);
    m_apiClient->deleteWorkspace(m_workspaceSlug,
        [this](bool success, const ApiError &error) {
            onDeleteFinished(success, error);
        });
}

void GeneralAppearanceTab::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString border = ThemeColors::border(dark).name();

    setStyleSheet(QStringLiteral(R"(
        GeneralAppearanceTab {
            background-color: transparent;
        }
        QLineEdit {
            color: %1;
            border: 1px solid %2;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 13px;
        }
    )").arg(textPrimary, border));
}

void GeneralAppearanceTab::buildUi()
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(24);

    // Workspace name
    auto *nameControl = new QWidget(this);
    auto *nameLayout = new QHBoxLayout(nameControl);
    nameLayout->setContentsMargins(0, 0, 0, 0);
    nameLayout->setSpacing(8);

    m_nameEdit = new QLineEdit(this);
    m_nameEdit->setObjectName(QStringLiteral("workspaceNameEdit"));
    m_nameEdit->setMinimumWidth(280);
    nameLayout->addWidget(m_nameEdit);

    m_updateNameButton = new QPushButton(tr("Update"), this);
    m_updateNameButton->setObjectName(QStringLiteral("updateNameButton"));
    m_updateNameButton->setEnabled(false);
    connect(m_updateNameButton, &QPushButton::clicked,
            this, &GeneralAppearanceTab::onUpdateNameClicked);
    nameLayout->addWidget(m_updateNameButton);
    nameLayout->addStretch();

    auto *nameRow = new SettingRow(this);
    nameRow->setTitle(tr("Workspace name"));
    nameRow->setDescription(tr("The display name of this workspace."));
    nameRow->setControl(nameControl);
    rootLayout->addWidget(nameRow);

    // Suggested messages
    m_suggestedEditor = new SuggestedMessagesEditor(this);
    m_suggestedEditor->setObjectName(QStringLiteral("suggestedMessagesEditor"));
    connect(m_suggestedEditor, &SuggestedMessagesEditor::saveRequested,
            this, &GeneralAppearanceTab::onSaveSuggestionsClicked);
    rootLayout->addWidget(m_suggestedEditor);

    // Delete workspace
    auto *deleteRow = new SettingRow(this);
    deleteRow->setTitle(tr("Delete workspace"));
    deleteRow->setDescription(tr("Permanently delete this workspace and all its data."));

    m_deleteButton = new QPushButton(tr("Delete workspace"), this);
    m_deleteButton->setObjectName(QStringLiteral("deleteWorkspaceButton"));
    m_deleteButton->setEnabled(false);
    connect(m_deleteButton, &QPushButton::clicked,
            this, &GeneralAppearanceTab::onDeleteClicked);
    deleteRow->setControl(m_deleteButton);
    rootLayout->addWidget(deleteRow);

    rootLayout->addStretch();
}

void GeneralAppearanceTab::setLoading(bool loading)
{
    m_nameEdit->setEnabled(!loading);
    m_updateNameButton->setEnabled(!loading && !m_workspaceSlug.isEmpty());
}
