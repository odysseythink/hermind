#include "agent_config_tab.h"
#include "agent_config_state.h"
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
#include <QVBoxLayout>

AgentConfigTab::AgentConfigTab(QWidget *parent)
    : QWidget(parent)
{
    buildUi();
    applyTheme();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &AgentConfigTab::applyTheme);
}

AgentConfigTab::~AgentConfigTab() = default;

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

    m_blockSignals = false;
}

bool AgentConfigTab::isDirty() const
{
    return m_state.isDirty();
}

QJsonObject AgentConfigTab::buildUpdatePayload() const
{
    return m_state.buildUpdatePayload();
}

void AgentConfigTab::reset()
{
    m_blockSignals = true;

    m_state.reset();

    const int idx = m_providerCombo->findData(m_state.provider());
    m_providerCombo->setCurrentIndex(idx >= 0 ? idx : 0);

    refreshModelList();
    updateWarning();

    m_blockSignals = false;
}

void AgentConfigTab::onProviderChanged(int)
{
    if (m_blockSignals)
        return;

    const QString provider = m_providerCombo->currentData().toString();
    m_state.setProvider(provider);

    refreshModelList();
    updateWarning();

    emit dirtyChanged(m_state.isDirty());
}

void AgentConfigTab::onModelChanged(const QString &model)
{
    if (m_blockSignals)
        return;

    m_state.setModel(model.trimmed());
    updateWarning();

    emit dirtyChanged(m_state.isDirty());
}

void AgentConfigTab::buildUi()
{
    m_layout = new QVBoxLayout(this);
    m_layout->setContentsMargins(0, 0, 0, 0);
    m_layout->setSpacing(20);

    // Header
    auto *headerLayout = new QHBoxLayout();
    auto *title = new QLabel(tr("Agent Configuration"), this);
    QFont titleFont = title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    title->setFont(titleFont);
    headerLayout->addWidget(title);
    headerLayout->addStretch();
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
    m_modelSelector = new QComboBox(this);
    m_modelSelector->setObjectName(QStringLiteral("agentModelCombo"));
    m_modelSelector->setEditable(true);
    connect(m_modelSelector, &QComboBox::currentTextChanged,
            this, &AgentConfigTab::onModelChanged);

    auto *modelRow = new SettingRow(this);
    modelRow->setTitle(tr("Agent Model"));
    modelRow->setDescription(tr("Model used for agent reasoning and tool calls."));
    modelRow->setControl(m_modelSelector);
    m_layout->addWidget(modelRow);

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

void AgentConfigTab::refreshModelList()
{
    const QString provider = m_providerCombo->currentData().toString();
    const bool supportsSelection = LlmModelSelector::supportsModelSelection(provider);

    m_modelSelector->setVisible(supportsSelection);

    if (!supportsSelection) {
        m_state.setModel(QString());
        return;
    }

    m_blockSignals = true;

    const QString previousModel = m_modelSelector->currentText();
    const QStringList models = LlmModelSelector::modelsForProvider(provider, QStringList());

    m_modelSelector->clear();
    m_modelSelector->addItems(models);

    const QString workspaceModel = m_state.model();
    if (!workspaceModel.isEmpty()) {
        if (models.contains(workspaceModel)) {
            m_modelSelector->setCurrentText(workspaceModel);
        } else {
            // Custom model: prepend it so it appears in the list.
            m_modelSelector->insertItem(0, workspaceModel);
            m_modelSelector->setCurrentIndex(0);
        }
    } else if (models.contains(previousModel)) {
        m_modelSelector->setCurrentText(previousModel);
    }

    m_state.setModel(m_modelSelector->currentText().trimmed());

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
