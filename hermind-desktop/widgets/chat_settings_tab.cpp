#include "chat_settings_tab.h"
#include "llm_model_selector.h"
#include "llm_provider_info.h"
#include "setting_row.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QButtonGroup>
#include <QComboBox>
#include <QDoubleSpinBox>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPushButton>
#include <QSpinBox>
#include <QTextEdit>
#include <QVBoxLayout>

static const char *kDefaultRefusal = "There is no relevant information in this workspace to answer your query.";

ChatSettingsTab::ChatSettingsTab(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ChatSettingsTab::applyStyle);
}

void ChatSettingsTab::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_workspace = HermindWorkspace();
    m_systemLlmProvider.clear();
    m_defaultSystemPrompt.clear();
    m_pendingCustomModels.clear();
    m_hasChanges = false;

    if (!m_apiClient || slug.isEmpty()) {
        setLoading(false);
        updateHasChanges();
        return;
    }

    setLoading(true);
    loadWorkspace();

    m_apiClient->defaultSystemPrompt(
        [this](const QString &prompt, const ApiError &error) {
            onDefaultPromptLoaded(prompt, error);
        });
}

void ChatSettingsTab::loadWorkspace()
{
    m_loadingWorkspace = true;
    m_apiClient->getWorkspace(m_workspaceSlug,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceLoaded(workspace, message, error);
        });
}

void ChatSettingsTab::onWorkspaceLoaded(const HermindWorkspace &workspace,
                                        const QString &,
                                        const ApiError &error)
{
    m_loadingWorkspace = false;
    if (!error.isEmpty()) {
        setLoading(false);
        QMessageBox::warning(this, tr("Load failed"), error.message());
        return;
    }

    m_workspace = workspace;

    m_apiClient->systemKeys(
        [this](const QJsonObject &settings, const ApiError &error) {
            onSystemKeysLoaded(settings, error);
        });
}

void ChatSettingsTab::onSystemKeysLoaded(const QJsonObject &settings,
                                         const ApiError &error)
{
    if (!error.isEmpty()) {
        setLoading(false);
        QMessageBox::warning(this, tr("Load failed"), error.message());
        return;
    }

    m_systemLlmProvider = settings.value(QStringLiteral("LLMProvider")).toString();
    if (m_systemLlmProvider.isEmpty())
        m_systemLlmProvider = QStringLiteral("openai");

    // Provider
    const QString provider = m_workspace.chatProvider().value_or(QStringLiteral("default"));
    m_providerCombo->setCurrentIndex(m_providerCombo->findData(provider));

    // Mode
    const QString mode = m_workspace.chatMode().isEmpty() ? QStringLiteral("chat") : m_workspace.chatMode();
    for (QAbstractButton *btn : m_modeGroup->buttons()) {
        if (btn->property("modeId").toString() == mode)
            btn->setChecked(true);
    }

    // History
    m_historySpin->setValue(m_workspace.openAiHistory() > 0 ? m_workspace.openAiHistory() : 20);

    // Temperature
    const double recommended = LlmModelSelector::recommendedTemperature(
        provider == QStringLiteral("default") ? m_systemLlmProvider : provider);
    m_tempSpin->setValue(m_workspace.openAiTemp().value_or(recommended));

    // Prompt
    const QString prompt = m_workspace.openAiPrompt().value_or(m_defaultSystemPrompt);
    m_promptEdit->setPlainText(prompt);

    // Refusal
    m_refusalEdit->setPlainText(m_workspace.queryRefusalResponse().value_or(QString::fromUtf8(kDefaultRefusal)));

    // Model
    updateModelSelector();

    setLoading(false);
    m_hasChanges = false;
    updateHasChanges();
}

void ChatSettingsTab::onDefaultPromptLoaded(const QString &prompt,
                                            const ApiError &error)
{
    if (!error.isEmpty())
        return;
    m_defaultSystemPrompt = prompt;
    if (m_promptEdit->toPlainText().isEmpty() && !prompt.isEmpty())
        m_promptEdit->setPlainText(prompt);
}

void ChatSettingsTab::onCustomModelsLoaded(const QStringList &models,
                                           const ApiError &error)
{
    if (!error.isEmpty()) {
        m_pendingCustomModels.clear();
    } else {
        m_pendingCustomModels = models;
    }

    const QString provider = currentProvider();
    const QStringList allModels = LlmModelSelector::modelsForProvider(provider, m_pendingCustomModels);

    const QString previousModel = m_modelCombo->currentText();
    m_modelCombo->clear();
    m_modelCombo->addItems(allModels);

    const QString workspaceModel = m_workspace.chatModel().value_or(QString());
    if (!workspaceModel.isEmpty() && allModels.contains(workspaceModel))
        m_modelCombo->setCurrentText(workspaceModel);
    else if (allModels.contains(previousModel))
        m_modelCombo->setCurrentText(previousModel);
}

void ChatSettingsTab::onWorkspaceUpdated(const HermindWorkspace &workspace,
                                         const QString &,
                                         const ApiError &error)
{
    m_saveButton->setEnabled(true);
    if (!error.isEmpty()) {
        QMessageBox::warning(this, tr("Save failed"), error.message());
        return;
    }

    m_workspace = workspace;
    m_hasChanges = false;
    updateHasChanges();
    emit workspaceUpdated(workspace);
}

void ChatSettingsTab::onProviderChanged(int)
{
    updateModelSelector();
    m_hasChanges = true;
    updateHasChanges();
}

void ChatSettingsTab::onModeChanged()
{
    m_hasChanges = true;
    updateHasChanges();
}

void ChatSettingsTab::onFieldEdited()
{
    m_hasChanges = true;
    updateHasChanges();
}

void ChatSettingsTab::onSaveClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    m_saveButton->setEnabled(false);
    m_apiClient->updateWorkspace(m_workspaceSlug, collectFields(),
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceUpdated(workspace, message, error);
        });
}

void ChatSettingsTab::onResetPromptClicked()
{
    if (!m_defaultSystemPrompt.isEmpty()) {
        m_promptEdit->setPlainText(m_defaultSystemPrompt);
        m_hasChanges = true;
        updateHasChanges();
    } else {
        m_apiClient->defaultSystemPrompt(
            [this](const QString &prompt, const ApiError &error) {
                if (!error.isEmpty())
                    return;
                m_defaultSystemPrompt = prompt;
                m_promptEdit->setPlainText(prompt);
                m_hasChanges = true;
                updateHasChanges();
            });
    }
}

void ChatSettingsTab::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString border = ThemeColors::border(dark).name();

    setStyleSheet(QStringLiteral(R"(
        ChatSettingsTab {
            background-color: transparent;
        }
        QComboBox, QSpinBox, QDoubleSpinBox, QLineEdit {
            color: %1;
            border: 1px solid %2;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 13px;
            min-width: 200px;
        }
        QTextEdit {
            color: %1;
            border: 1px solid %2;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 13px;
        }
        QPushButton {
            border-radius: 8px;
            padding: 8px 16px;
        }
    )").arg(textPrimary, border));
}

void ChatSettingsTab::buildUi()
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(20);

    // Header + save
    auto *headerLayout = new QHBoxLayout();
    auto *title = new QLabel(tr("Chat Settings"), this);
    QFont titleFont = title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    title->setFont(titleFont);
    headerLayout->addWidget(title);
    headerLayout->addStretch();

    m_saveButton = new QPushButton(tr("Update Workspace"), this);
    m_saveButton->setObjectName(QStringLiteral("updateWorkspaceButton"));
    m_saveButton->setVisible(false);
    connect(m_saveButton, &QPushButton::clicked,
            this, &ChatSettingsTab::onSaveClicked);
    headerLayout->addWidget(m_saveButton);
    rootLayout->addLayout(headerLayout);

    // Provider
    m_providerCombo = new QComboBox(this);
    m_providerCombo->setObjectName(QStringLiteral("providerCombo"));
    for (const LlmProviderInfo &info : LlmProviderInfo::all()) {
        m_providerCombo->addItem(info.name, info.id);
    }
    connect(m_providerCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &ChatSettingsTab::onProviderChanged);

    auto *providerRow = new SettingRow(this);
    providerRow->setTitle(tr("LLM Provider"));
    providerRow->setDescription(tr("Select the LLM provider for this workspace."));
    providerRow->setControl(m_providerCombo);
    rootLayout->addWidget(providerRow);

    // Model
    m_modelCombo = new QComboBox(this);
    m_modelCombo->setObjectName(QStringLiteral("modelCombo"));
    m_modelCombo->setEditable(true);
    connect(m_modelCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &ChatSettingsTab::onFieldEdited);
    connect(m_modelCombo->lineEdit(), &QLineEdit::textEdited,
            this, &ChatSettingsTab::onFieldEdited);

    m_modelLineEdit = new QLineEdit(this);
    m_modelLineEdit->setObjectName(QStringLiteral("modelLineEdit"));
    m_modelLineEdit->setPlaceholderText(tr("Enter model name"));
    connect(m_modelLineEdit, &QLineEdit::textEdited,
            this, &ChatSettingsTab::onFieldEdited);

    auto *modelRow = new SettingRow(this);
    modelRow->setTitle(tr("Model"));
    modelRow->setDescription(tr("Select or enter the model for the chosen provider."));
    auto *modelStack = new QWidget(this);
    auto *modelStackLayout = new QVBoxLayout(modelStack);
    modelStackLayout->setContentsMargins(0, 0, 0, 0);
    modelStackLayout->addWidget(m_modelCombo);
    modelStackLayout->addWidget(m_modelLineEdit);
    modelRow->setControl(modelStack);
    rootLayout->addWidget(modelRow);

    // Mode
    m_modeGroup = new QButtonGroup(this);
    m_modeGroup->setExclusive(true);
    auto *modeWidget = new QWidget(this);
    auto *modeLayout = new QHBoxLayout(modeWidget);
    modeLayout->setContentsMargins(0, 0, 0, 0);
    for (const QString &modeId : { QStringLiteral("automatic"), QStringLiteral("chat"), QStringLiteral("query") }) {
        auto *btn = new QPushButton(modeId, modeWidget);
        btn->setObjectName(QStringLiteral("modeButton_") + modeId);
        btn->setProperty("modeId", modeId);
        btn->setCheckable(true);
        btn->setAutoExclusive(true);
        m_modeGroup->addButton(btn);
        modeLayout->addWidget(btn);
        connect(btn, &QPushButton::toggled,
                this, &ChatSettingsTab::onModeChanged);
    }
    modeLayout->addStretch();

    auto *modeRow = new SettingRow(this);
    modeRow->setTitle(tr("Chat Mode"));
    modeRow->setDescription(tr("Automatic: decide chat or query per message. Chat: conversational. Query: RAG-only."));
    modeRow->setControl(modeWidget);
    rootLayout->addWidget(modeRow);

    // History
    m_historySpin = new QSpinBox(this);
    m_historySpin->setObjectName(QStringLiteral("historySpin"));
    m_historySpin->setRange(1, 9999);
    m_historySpin->setValue(20);
    connect(m_historySpin, QOverload<int>::of(&QSpinBox::valueChanged),
            this, &ChatSettingsTab::onFieldEdited);

    auto *historyRow = new SettingRow(this);
    historyRow->setTitle(tr("Chat History"));
    historyRow->setDescription(tr("Number of previous messages to include in context."));
    historyRow->setControl(m_historySpin);
    rootLayout->addWidget(historyRow);

    // Temperature
    m_tempSpin = new QDoubleSpinBox(this);
    m_tempSpin->setObjectName(QStringLiteral("temperatureSpin"));
    m_tempSpin->setRange(0.0, 2.0);
    m_tempSpin->setSingleStep(0.1);
    m_tempSpin->setDecimals(1);
    m_tempSpin->setValue(0.7);
    connect(m_tempSpin, QOverload<double>::of(&QDoubleSpinBox::valueChanged),
            this, &ChatSettingsTab::onFieldEdited);

    auto *tempRow = new SettingRow(this);
    tempRow->setTitle(tr("Temperature"));
    tempRow->setDescription(tr("Lower values produce more deterministic responses."));
    tempRow->setControl(m_tempSpin);
    rootLayout->addWidget(tempRow);

    // Prompt
    m_promptEdit = new QTextEdit(this);
    m_promptEdit->setObjectName(QStringLiteral("promptEdit"));
    m_promptEdit->setMinimumHeight(120);
    m_promptEdit->setPlaceholderText(tr("System prompt for this workspace..."));
    connect(m_promptEdit, &QTextEdit::textChanged,
            this, &ChatSettingsTab::onFieldEdited);

    m_resetPromptButton = new QPushButton(tr("Restore to Default"), this);
    m_resetPromptButton->setObjectName(QStringLiteral("resetPromptButton"));
    connect(m_resetPromptButton, &QPushButton::clicked,
            this, &ChatSettingsTab::onResetPromptClicked);

    auto *promptRow = new SettingRow(this);
    promptRow->setTitle(tr("System Prompt"));
    promptRow->setDescription(tr("The default system prompt used for conversations."));
    auto *promptControl = new QWidget(this);
    auto *promptControlLayout = new QVBoxLayout(promptControl);
    promptControlLayout->setContentsMargins(0, 0, 0, 0);
    promptControlLayout->addWidget(m_promptEdit);
    promptControlLayout->addWidget(m_resetPromptButton, 0, Qt::AlignLeft);
    promptRow->setControl(promptControl);
    rootLayout->addWidget(promptRow);

    // Refusal
    m_refusalEdit = new QTextEdit(this);
    m_refusalEdit->setObjectName(QStringLiteral("refusalEdit"));
    m_refusalEdit->setMinimumHeight(60);
    m_refusalEdit->setPlaceholderText(tr("The text returned in query mode when no relevant context is found."));
    connect(m_refusalEdit, &QTextEdit::textChanged,
            this, &ChatSettingsTab::onFieldEdited);

    auto *refusalRow = new SettingRow(this);
    refusalRow->setTitle(tr("Query Refusal Response"));
    refusalRow->setDescription(tr("Shown to the user when no relevant context is found in query mode."));
    refusalRow->setControl(m_refusalEdit);
    rootLayout->addWidget(refusalRow);

    rootLayout->addStretch();
}

void ChatSettingsTab::setLoading(bool loading)
{
    m_providerCombo->setEnabled(!loading);
    m_modelCombo->setEnabled(!loading);
    m_modelLineEdit->setEnabled(!loading);
    for (QAbstractButton *btn : m_modeGroup->buttons())
        btn->setEnabled(!loading);
    m_historySpin->setEnabled(!loading);
    m_tempSpin->setEnabled(!loading);
    m_promptEdit->setEnabled(!loading);
    m_refusalEdit->setEnabled(!loading);
    m_resetPromptButton->setEnabled(!loading);
}

void ChatSettingsTab::loadCustomModels(const QString &provider)
{
    if (provider == QStringLiteral("default") || !LlmModelSelector::supportsModelSelection(provider))
        return;

    m_apiClient->customModels(provider,
        [this](const QStringList &models, const ApiError &error) {
            onCustomModelsLoaded(models, error);
        });
}

void ChatSettingsTab::updateModelSelector()
{
    const QString provider = currentProvider();
    const bool manual = LlmModelSelector::isManualModelInput(provider);
    const bool supports = LlmModelSelector::supportsModelSelection(provider);

    m_modelCombo->setVisible(supports);
    m_modelLineEdit->setVisible(manual);

    if (manual) {
        m_modelLineEdit->setText(m_workspace.chatModel().value_or(QString()));
    } else if (supports) {
        loadCustomModels(provider);
    } else {
        m_modelCombo->clear();
    }
}

void ChatSettingsTab::updateHasChanges()
{
    if (m_loadingWorkspace) {
        m_saveButton->setVisible(false);
        return;
    }

    m_saveButton->setVisible(m_hasChanges);
    m_saveButton->setEnabled(m_hasChanges);
    emit hasChangesChanged(m_hasChanges);
}

QJsonObject ChatSettingsTab::collectFields() const
{
    QJsonObject body;
    const QString provider = currentProvider();
    body.insert(QStringLiteral("chatProvider"), provider);

    const QString model = currentModel();
    if (!model.isEmpty())
        body.insert(QStringLiteral("chatModel"), model);

    body.insert(QStringLiteral("chatMode"), m_modeGroup->checkedButton()
                ? m_modeGroup->checkedButton()->property("modeId").toString()
                : QStringLiteral("chat"));
    body.insert(QStringLiteral("openAiHistory"), m_historySpin->value());
    body.insert(QStringLiteral("openAiTemp"), m_tempSpin->value());
    body.insert(QStringLiteral("openAiPrompt"), m_promptEdit->toPlainText());
    body.insert(QStringLiteral("queryRefusalResponse"), m_refusalEdit->toPlainText());
    return body;
}

QString ChatSettingsTab::currentProvider() const
{
    return m_providerCombo->currentData().toString();
}

QString ChatSettingsTab::currentModel() const
{
    const QString provider = currentProvider();
    if (provider == QStringLiteral("default"))
        return QString();
    if (LlmModelSelector::isManualModelInput(provider))
        return m_modelLineEdit->text().trimmed();
    if (LlmModelSelector::supportsModelSelection(provider))
        return m_modelCombo->currentText().trimmed();
    return QString();
}
