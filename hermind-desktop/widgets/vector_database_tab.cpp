#include "vector_database_tab.h"
#include "setting_row.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QComboBox>
#include <QHBoxLayout>
#include <QLabel>
#include <QMessageBox>
#include <QPushButton>
#include <QSpinBox>
#include <QVBoxLayout>

VectorDatabaseTab::VectorDatabaseTab(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &VectorDatabaseTab::applyStyle);
}

void VectorDatabaseTab::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_workspace = HermindWorkspace();
    m_vectorDb.clear();
    m_hasChanges = false;

    if (!m_apiClient || slug.isEmpty()) {
        updateSearchModeVisibility();
        updateHasChanges();
        return;
    }

    setLoading(true);
    m_apiClient->getWorkspace(slug,
        [this](const HermindWorkspace &workspace, const QString &message,
               const ApiError &error) {
            onWorkspaceLoaded(workspace, message, error);
        });

    m_apiClient->systemKeys(
        [this](const QJsonObject &settings, const ApiError &error) {
            onSystemKeysLoaded(settings, error);
        });
}

void VectorDatabaseTab::onWorkspaceLoaded(const HermindWorkspace &workspace,
                                          const QString &,
                                          const ApiError &error)
{
    if (!error.isEmpty()) {
        setLoading(false);
        QMessageBox::warning(this, tr("Load failed"), error.message());
        return;
    }

    m_workspace = workspace;
    m_identifierLabel->setText(workspace.slug());

    m_topNSpin->setValue(workspace.topN().value_or(4));

    const double threshold = workspace.similarityThreshold().value_or(0.25);
    const int idx = m_thresholdCombo->findData(threshold);
    if (idx >= 0)
        m_thresholdCombo->setCurrentIndex(idx);

    const QString mode = workspace.vectorSearchMode().isEmpty()
                         ? QStringLiteral("default") : workspace.vectorSearchMode();
    const int modeIdx = m_searchModeCombo->findData(mode);
    if (modeIdx >= 0)
        m_searchModeCombo->setCurrentIndex(modeIdx);

    setLoading(false);
    updateSearchModeVisibility();
    m_hasChanges = false;
    updateHasChanges();
    loadVectorCount();
}

void VectorDatabaseTab::onSystemKeysLoaded(const QJsonObject &settings,
                                           const ApiError &error)
{
    if (!error.isEmpty())
        return;

    m_vectorDb = settings.value(QStringLiteral("VectorDB")).toString().toLower();
    if (m_vectorDb.isEmpty())
        m_vectorDb = QStringLiteral("lancedb");
    updateSearchModeVisibility();
    loadVectorCount();
}

void VectorDatabaseTab::onVectorCountLoaded(int count,
                                            const ApiError &error)
{
    if (!error.isEmpty()) {
        m_vectorCountLabel->setText(tr("Unable to fetch"));
        return;
    }
    m_vectorCountLabel->setText(QString::number(count));
}

void VectorDatabaseTab::onWorkspaceUpdated(const HermindWorkspace &workspace,
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
    loadVectorCount();
}

void VectorDatabaseTab::onWipeFinished(bool success, const ApiError &error)
{
    m_resetButton->setEnabled(true);
    if (!success || !error.isEmpty()) {
        QMessageBox::warning(this, tr("Reset failed"),
                             error.isEmpty() ? tr("Please try again later") : error.message());
        return;
    }

    QMessageBox::information(this, tr("Reset complete"), tr("Vector database has been reset."));
    loadVectorCount();
}

void VectorDatabaseTab::onFieldEdited()
{
    m_hasChanges = true;
    updateHasChanges();
}

void VectorDatabaseTab::onSaveClicked()
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

void VectorDatabaseTab::onResetClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    const int ret = QMessageBox::question(
        this,
        tr("Reset Vector Database"),
        tr("Are you sure you want to clear the vector database for workspace %1? This action cannot be undone.").arg(m_workspace.name()),
        QMessageBox::Yes | QMessageBox::No,
        QMessageBox::No);

    if (ret != QMessageBox::Yes)
        return;

    m_resetButton->setEnabled(false);
    m_apiClient->wipeVectorDb(m_workspaceSlug,
        [this](bool success, const ApiError &error) {
            onWipeFinished(success, error);
        });
}

void VectorDatabaseTab::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString border = ThemeColors::border(dark).name();
    const QString danger = QColor(220, 53, 69).name();

    setStyleSheet(QStringLiteral(R"(
        VectorDatabaseTab {
            background-color: transparent;
        }
        QComboBox, QSpinBox {
            color: %1;
            border: 1px solid %2;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 13px;
            min-width: 200px;
        }
        QPushButton#resetVectorDbButton {
            color: white;
            background-color: %3;
            border-radius: 8px;
            padding: 8px 16px;
        }
    )").arg(textPrimary, border, danger));
}

void VectorDatabaseTab::buildUi()
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(20);

    // Header + save
    auto *headerLayout = new QHBoxLayout();
    auto *title = new QLabel(tr("Vector Database"), this);
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
            this, &VectorDatabaseTab::onSaveClicked);
    headerLayout->addWidget(m_saveButton);
    rootLayout->addLayout(headerLayout);

    // Identifier + Vector count (horizontal)
    auto *topRow = new QWidget(this);
    auto *topRowLayout = new QHBoxLayout(topRow);
    topRowLayout->setContentsMargins(0, 0, 0, 0);
    topRowLayout->setSpacing(24);

    auto *identifierRow = new SettingRow(this);
    identifierRow->setTitle(tr("Vector DB Identifier"));
    m_identifierLabel = new QLabel(this);
    m_identifierLabel->setObjectName(QStringLiteral("vectorDbIdentifier"));
    identifierRow->setControl(m_identifierLabel);
    topRowLayout->addWidget(identifierRow);

    auto *countRow = new SettingRow(this);
    countRow->setTitle(tr("Total Vectors"));
    m_vectorCountLabel = new QLabel(tr("Loading..."), this);
    m_vectorCountLabel->setObjectName(QStringLiteral("vectorCountLabel"));
    countRow->setControl(m_vectorCountLabel);
    topRowLayout->addWidget(countRow);
    topRowLayout->addStretch();

    rootLayout->addWidget(topRow);

    // Search mode
    m_searchModeCombo = new QComboBox(this);
    m_searchModeCombo->setObjectName(QStringLiteral("searchModeCombo"));
    m_searchModeCombo->addItem(tr("Default"), QStringLiteral("default"));
    m_searchModeCombo->addItem(tr("Accuracy Optimized"), QStringLiteral("rerank"));
    connect(m_searchModeCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &VectorDatabaseTab::onFieldEdited);

    auto *searchModeRow = new SettingRow(this);
    searchModeRow->setObjectName(QStringLiteral("searchModeRow"));
    searchModeRow->setTitle(tr("Search Preference"));
    searchModeRow->setDescription(tr("Accuracy Optimized may be slower but returns more relevant results."));
    searchModeRow->setControl(m_searchModeCombo);
    rootLayout->addWidget(searchModeRow);

    // Max context snippets
    m_topNSpin = new QSpinBox(this);
    m_topNSpin->setObjectName(QStringLiteral("topNSpin"));
    m_topNSpin->setRange(1, 200);
    m_topNSpin->setValue(4);
    connect(m_topNSpin, QOverload<int>::of(&QSpinBox::valueChanged),
            this, &VectorDatabaseTab::onFieldEdited);

    auto *topNRow = new SettingRow(this);
    topNRow->setTitle(tr("Max Context Snippets"));
    topNRow->setDescription(tr("Number of context snippets to inject into the prompt."));
    topNRow->setControl(m_topNSpin);
    rootLayout->addWidget(topNRow);

    // Similarity threshold
    m_thresholdCombo = new QComboBox(this);
    m_thresholdCombo->setObjectName(QStringLiteral("thresholdCombo"));
    m_thresholdCombo->addItem(tr("No threshold"), 0.0);
    m_thresholdCombo->addItem(tr("Low (0.25)"), 0.25);
    m_thresholdCombo->addItem(tr("Medium (0.5)"), 0.5);
    m_thresholdCombo->addItem(tr("High (0.75)"), 0.75);
    connect(m_thresholdCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &VectorDatabaseTab::onFieldEdited);

    auto *thresholdRow = new SettingRow(this);
    thresholdRow->setTitle(tr("Document Similarity Threshold"));
    thresholdRow->setDescription(tr("Higher values require stronger similarity to include a document."));
    thresholdRow->setControl(m_thresholdCombo);
    rootLayout->addWidget(thresholdRow);

    // Reset vector DB
    m_resetButton = new QPushButton(tr("Reset Vector Database"), this);
    m_resetButton->setObjectName(QStringLiteral("resetVectorDbButton"));
    connect(m_resetButton, &QPushButton::clicked,
            this, &VectorDatabaseTab::onResetClicked);

    auto *resetRow = new SettingRow(this);
    resetRow->setTitle(tr("Reset Vector Database"));
    resetRow->setDescription(tr("Remove all vectors for this workspace. Documents remain."));
    resetRow->setControl(m_resetButton);
    rootLayout->addWidget(resetRow);

    rootLayout->addStretch();
}

void VectorDatabaseTab::setLoading(bool loading)
{
    m_searchModeCombo->setEnabled(!loading);
    m_topNSpin->setEnabled(!loading);
    m_thresholdCombo->setEnabled(!loading);
    m_resetButton->setEnabled(!loading);
}

void VectorDatabaseTab::updateSearchModeVisibility()
{
    const bool visible = (m_vectorDb == QStringLiteral("lancedb"));
    auto *searchModeRow = findChild<SettingRow *>(QStringLiteral("searchModeRow"));
    if (searchModeRow)
        searchModeRow->setVisible(visible);
}

void VectorDatabaseTab::updateHasChanges()
{
    m_saveButton->setVisible(m_hasChanges);
    m_saveButton->setEnabled(m_hasChanges);
    emit hasChangesChanged(m_hasChanges);
}

void VectorDatabaseTab::loadVectorCount()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    m_vectorCountLabel->setText(tr("Loading..."));
    m_apiClient->systemVectors(m_workspaceSlug,
        [this](int count, const ApiError &error) {
            onVectorCountLoaded(count, error);
        });
}

QJsonObject VectorDatabaseTab::collectFields() const
{
    QJsonObject body;
    body.insert(QStringLiteral("topN"), m_topNSpin->value());
    body.insert(QStringLiteral("similarityThreshold"), m_thresholdCombo->currentData().toDouble());

    auto *searchModeRow = findChild<SettingRow *>(QStringLiteral("searchModeRow"));
    if (searchModeRow && searchModeRow->isVisible())
        body.insert(QStringLiteral("vectorSearchMode"), m_searchModeCombo->currentData().toString());

    return body;
}
