#include "agent_config_tab.h"
#include "agent_config_state.h"
#include "auth_manager.h"
#include "hermind_api_client.h"
#include "hermind_workspace.h"
#include "llm_model_selector.h"
#include "llm_provider_info.h"
#include "setting_row.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QComboBox>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPushButton>
#include <QVBoxLayout>

AgentConfigTab::AgentConfigTab(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyTheme();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &AgentConfigTab::applyTheme);
    connect(&AuthManager::instance(), &AuthManager::userChanged,
            this, [this](const HermindUser &) {
                updateSkillsButton();
            });
    updateSkillsButton();
}

QString AgentConfigTab::workspaceSlug() const
{
    return m_workspaceSlug;
}

void AgentConfigTab::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_state = AgentConfigState();
    reload();
}

void AgentConfigTab::reload()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty()) {
        emit configLoaded(false);
        return;
    }

    loadWorkspace();
}

bool AgentConfigTab::isDirty() const
{
    return m_state.isDirty();
}

QJsonObject AgentConfigTab::buildUpdatePayload() const
{
    return m_state.buildUpdatePayload();
}

void AgentConfigTab::loadFromWorkspace(const HermindWorkspace &workspace)
{
    m_blockSignals = true;

    const QString provider = workspace.agentProvider().value_or(QStringLiteral("default"));
    const QString model = workspace.agentModel().value_or(QString());

    m_state.setOriginalProvider(provider);
    m_state.setOriginalModel(model);
    m_state.setProvider(provider);
    m_state.setModel(model);

    const int idx = m_providerCombo->findData(provider);
    m_providerCombo->setCurrentIndex(idx >= 0 ? idx : 0);

    refreshModelList();
    updateWarning();
    updateSaveButton();

    m_blockSignals = false;
}

void AgentConfigTab::onWorkspaceLoaded(const HermindWorkspace &workspace,
                                       const QString &,
                                       const ApiError &error)
{
    if (!error.isEmpty() || workspace.id() == 0) {
        emit configLoaded(false);
        return;
    }

    loadFromWorkspace(workspace);
    loadCustomModels();
    emit configLoaded(true);
}

void AgentConfigTab::onModelsLoaded(const QStringList &models,
                                    const ApiError &error)
{
    if (error.isEmpty()) {
        m_blockSignals = true;

        const QString previousModel = m_modelCombo->currentText();
        const QStringList allModels = LlmModelSelector::modelsForProvider(
            m_state.provider(), models);

        m_modelCombo->clear();
        for (const QString &model : allModels) {
            if (m_state.isModelSupported(m_state.provider(), model))
                m_modelCombo->addItem(model);
        }

        const QString workspaceModel = m_state.model();
        if (!workspaceModel.isEmpty()) {
            if (allModels.contains(workspaceModel)) {
                m_modelCombo->setCurrentText(workspaceModel);
            } else if (m_state.isModelSupported(m_state.provider(), workspaceModel)) {
                m_modelCombo->insertItem(0, workspaceModel);
                m_modelCombo->setCurrentIndex(0);
            }
        } else if (allModels.contains(previousModel)) {
            m_modelCombo->setCurrentText(previousModel);
        }

        m_state.setModel(m_modelCombo->currentText().trimmed());
        updateWarning();
        updateSaveButton();

        m_blockSignals = false;
    }
}

void AgentConfigTab::onUpdateFinished(const HermindWorkspace &workspace,
                                      const QString &,
                                      const ApiError &error)
{
    m_saveButton->setEnabled(true);
    if (!error.isEmpty()) {
        QMessageBox::warning(this, tr("Update failed"), error.message());
        return;
    }

    m_state.setOriginalProvider(m_state.provider());
    m_state.setOriginalModel(m_state.model());
    updateSaveButton();
    emit workspaceUpdated(workspace);
}

void AgentConfigTab::onProviderChanged(int)
{
    if (m_blockSignals)
        return;

    const QString provider = m_providerCombo->currentData().toString();
    m_state.setProvider(provider);

    refreshModelList();
    loadCustomModels();
    updateWarning();
    updateSaveButton();

    emit dirtyChanged(m_state.isDirty());
    emit configLoaded(true);
}

void AgentConfigTab::onModelChanged(const QString &model)
{
    if (m_blockSignals)
        return;

    m_state.setModel(model.trimmed());
    updateWarning();
    updateSaveButton();

    emit dirtyChanged(m_state.isDirty());
}

void AgentConfigTab::onSaveClicked()
{
    if (!m_state.isDirty() || !m_apiClient || m_workspaceSlug.isEmpty())
        return;

    m_saveButton->setEnabled(false);
    m_apiClient->updateWorkspace(m_workspaceSlug, m_state.buildUpdatePayload(),
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onUpdateFinished(workspace, message, error);
        });
}

void AgentConfigTab::buildUi()
{
    m_layout = new QVBoxLayout(this);
    m_layout->setContentsMargins(0, 0, 0, 0);
    m_layout->setSpacing(20);

    // Header + save
    auto *headerLayout = new QHBoxLayout();
    auto *title = new QLabel(tr("Agent Configuration"), this);
    QFont titleFont = title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    title->setFont(titleFont);
    headerLayout->addWidget(title);
    headerLayout->addStretch();

    m_saveButton = new QPushButton(tr("Update Workspace"), this);
    m_saveButton->setObjectName(QStringLiteral("agentConfigSaveButton"));
    m_saveButton->setEnabled(false);
    connect(m_saveButton, &QPushButton::clicked,
            this, &AgentConfigTab::onSaveClicked);
    headerLayout->addWidget(m_saveButton);
    m_layout->addLayout(headerLayout);

    // Provider row
    m_providerCombo = new QComboBox(this);
    m_providerCombo->setObjectName(QStringLiteral("agentProviderCombo"));
    for (const LlmProviderInfo &info : LlmProviderInfo::all()) {
        m_providerCombo->addItem(info.name, info.id);
    }
    connect(m_providerCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &AgentConfigTab::onProviderChanged);

    auto *providerRow = new SettingRow(this);
    providerRow->setTitle(tr("Agent Provider"));
    providerRow->setDescription(tr("Default LLM provider used by agents in this workspace."));
    providerRow->setControl(m_providerCombo);
    m_layout->addWidget(providerRow);

    // Model row
    m_modelCombo = new QComboBox(this);
    m_modelCombo->setObjectName(QStringLiteral("agentModelCombo"));
    m_modelCombo->setEditable(true);
    connect(m_modelCombo, &QComboBox::currentTextChanged,
            this, &AgentConfigTab::onModelChanged);

    auto *modelRow = new SettingRow(this);
    modelRow->setTitle(tr("Agent Model"));
    modelRow->setDescription(tr("Model used for agent reasoning and tool calls."));
    modelRow->setControl(m_modelCombo);
    m_layout->addWidget(modelRow);

    // Agent skills entry (global settings; web: /settings/agents)
    auto *skillsRow = new SettingRow(this);
    skillsRow->setObjectName(QStringLiteral("agentSkillsRow"));
    skillsRow->setTitle(tr("Agent Skills"));
    skillsRow->setDescription(tr("Customize the default agent's capabilities by enabling or disabling specific skills. Applied across all workspaces."));

    m_agentSkillsButton = new QPushButton(tr("Configure Agent Skills"), this);
    m_agentSkillsButton->setObjectName(QStringLiteral("agentSkillsButton"));
    connect(m_agentSkillsButton, &QPushButton::clicked,
            this, &AgentConfigTab::agentSkillsRequested);
    skillsRow->setControl(m_agentSkillsButton);
    m_skillsRow = skillsRow;
    m_layout->addWidget(skillsRow);

    // Warning label
    m_warningLabel = new QLabel(this);
    m_warningLabel->setObjectName(QStringLiteral("agentPerformanceWarning"));
    m_warningLabel->setWordWrap(true);
    m_warningLabel->setVisible(false);
    m_layout->addWidget(m_warningLabel);

    m_layout->addStretch();
}

void AgentConfigTab::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString border = ThemeColors::border(dark).name();
    const QString warningText = dark ? QStringLiteral("#fbbf24") : QStringLiteral("#b45309");
    const QString warningBackground = dark ? QStringLiteral("#451a03") : QStringLiteral("#fffbeb");

    setStyleSheet(QStringLiteral(R"(
        AgentConfigTab {
            background-color: transparent;
        }
        QComboBox {
            color: %1;
            border: 1px solid %2;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 13px;
            min-width: 200px;
        }
        #agentPerformanceWarning {
            color: %3;
            background-color: %4;
            border: 1px solid %3;
            border-radius: 8px;
            padding: 10px 12px;
            font-size: 13px;
        }
    )").arg(textPrimary, border, warningText, warningBackground));
}

void AgentConfigTab::loadWorkspace()
{
    m_apiClient->getWorkspace(m_workspaceSlug,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceLoaded(workspace, message, error);
        });
}

void AgentConfigTab::loadCustomModels()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    const QString provider = m_state.provider();
    if (provider == QStringLiteral("default") ||
        !LlmModelSelector::supportsModelSelection(provider)) {
        return;
    }

    m_apiClient->customModels(provider,
        [this](const QStringList &models, const ApiError &error) {
            onModelsLoaded(models, error);
        });
}

void AgentConfigTab::refreshModelList()
{
    const QString provider = m_providerCombo->currentData().toString();
    const bool supportsSelection = LlmModelSelector::supportsModelSelection(provider);

    m_modelCombo->setVisible(supportsSelection);

    if (!supportsSelection) {
        m_state.setModel(QString());
        return;
    }

    m_blockSignals = true;

    const QString previousModel = m_modelCombo->currentText();
    const QStringList models = LlmModelSelector::modelsForProvider(provider, QStringList());

    m_modelCombo->clear();
    for (const QString &model : models) {
        if (m_state.isModelSupported(provider, model))
            m_modelCombo->addItem(model);
    }

    const QString workspaceModel = m_state.model();
    if (!workspaceModel.isEmpty()) {
        if (models.contains(workspaceModel)) {
            m_modelCombo->setCurrentText(workspaceModel);
        } else if (m_state.isModelSupported(provider, workspaceModel)) {
            m_modelCombo->insertItem(0, workspaceModel);
            m_modelCombo->setCurrentIndex(0);
        }
    } else if (models.contains(previousModel)) {
        m_modelCombo->setCurrentText(previousModel);
    }

    m_state.setModel(m_modelCombo->currentText().trimmed());

    m_blockSignals = false;
}

void AgentConfigTab::updateWarning()
{
    const bool warning = m_state.isPerformanceWarning();
    const bool supported = m_state.isModelSupported(m_state.provider(), m_state.model());

    if (!supported) {
        m_warningLabel->setText(tr("The selected model is not supported for agent use with this provider."));
        m_warningLabel->setVisible(true);
    } else if (warning) {
        m_warningLabel->setText(tr("Local providers may have slower or less reliable agent performance."));
        m_warningLabel->setVisible(true);
    } else {
        m_warningLabel->setVisible(false);
    }

    emit performanceWarningChanged(warning || !supported);
}

void AgentConfigTab::updateSaveButton()
{
    m_saveButton->setEnabled(m_state.isDirty());
    updateSkillsButton();
}

void AgentConfigTab::updateSkillsButton()
{
    if (!m_skillsRow)
        return;

    // Same visibility rule as the web UI: only admins (or single-user mode,
    // where there is no user record) may manage global agent skills, and the
    // entry is hidden while there are unsaved changes.
    const HermindUser user = AuthManager::instance().currentUser();
    const bool canManage = user.id() == 0 || user.role() == QStringLiteral("admin");
    m_skillsRow->setVisible(canManage && !m_state.isDirty());
}
