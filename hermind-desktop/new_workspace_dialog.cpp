#include "new_workspace_dialog.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QDialogButtonBox>
#include <QPushButton>

#include "theme_colors.h"
#include "theme_style_helper.h"
#include "hermind_api_client.h"
#include "api_response.h"

NewWorkspaceDialog::NewWorkspaceDialog(QWidget *parent)
    : QDialog(parent)
    , m_nameEdit(new QLineEdit(this))
    , m_errorLabel(new QLabel(this))
    , m_buttonBox(new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel, this))
{
    setWindowTitle(tr("New Workspace"));
    setMinimumWidth(400);
    setModal(true);

    auto *mainLayout = new QVBoxLayout(this);
    mainLayout->setSpacing(16);
    mainLayout->setContentsMargins(24, 24, 24, 24);

    auto *titleLabel = new QLabel(tr("New Workspace"), this);
    QFont titleFont = titleLabel->font();
    titleFont.setPointSize(14);
    titleFont.setBold(true);
    titleLabel->setFont(titleFont);
    mainLayout->addWidget(titleLabel);

    auto *formLayout = new QVBoxLayout();
    formLayout->setSpacing(8);

    auto *nameLabel = new QLabel(tr("Workspace name"), this);
    formLayout->addWidget(nameLabel);

    m_nameEdit->setPlaceholderText(tr("My Workspace"));
    m_nameEdit->setMinimumHeight(32);
    m_nameEdit->setClearButtonEnabled(true);
    formLayout->addWidget(m_nameEdit);

    m_errorLabel->setObjectName(QStringLiteral("errorLabel"));
    m_errorLabel->setWordWrap(true);
    m_errorLabel->setVisible(false);
    formLayout->addWidget(m_errorLabel);

    mainLayout->addLayout(formLayout);
    mainLayout->addStretch();

    m_buttonBox->setCenterButtons(false);
    m_saveButton = m_buttonBox->button(QDialogButtonBox::Save);
    m_saveButton->setText(tr("Save"));
    m_saveButton->setEnabled(false);
    mainLayout->addWidget(m_buttonBox);

    connect(m_nameEdit, &QLineEdit::textChanged, this, &NewWorkspaceDialog::onTextChanged);
    connect(m_buttonBox, &QDialogButtonBox::accepted, this, &NewWorkspaceDialog::onSaveClicked);
    connect(m_buttonBox, &QDialogButtonBox::rejected, this, &QDialog::reject);

    m_styleHelper = new ThemeStyleHelper(this, [this](QWidget *, bool dark) {
        applyStyle(dark);
    }, this);
}

void NewWorkspaceDialog::setApiClient(HermindApiClient *apiClient)
{
    m_apiClient = apiClient;
}

void NewWorkspaceDialog::onTextChanged()
{
    m_saveButton->setEnabled(!m_nameEdit->text().trimmed().isEmpty() && !m_loading);
    if (m_errorLabel->isVisible())
        m_errorLabel->setVisible(false);
}

void NewWorkspaceDialog::onSaveClicked()
{
    if (!m_apiClient) {
        setError(tr("API client is not available."));
        return;
    }

    const QString name = m_nameEdit->text().trimmed();
    if (name.isEmpty())
        return;

    setLoading(true);
    m_apiClient->createWorkspace(name,
        [this](const HermindWorkspace &workspace, const QString &message, const ApiError &error) {
            setLoading(false);
            if (!error.isEmpty() || workspace.slug().isEmpty()) {
                setError(message.isEmpty() ? error.message() : message);
                return;
            }
            emit workspaceCreated(workspace);
            accept();
        });
}

void NewWorkspaceDialog::setLoading(bool loading)
{
    m_loading = loading;
    m_nameEdit->setEnabled(!loading);
    m_saveButton->setEnabled(!loading && !m_nameEdit->text().trimmed().isEmpty());
    m_buttonBox->button(QDialogButtonBox::Cancel)->setEnabled(!loading);
    m_saveButton->setText(loading ? tr("Saving...") : tr("Save"));
}

void NewWorkspaceDialog::setError(const QString &message)
{
    m_errorLabel->setText(tr("Error: %1").arg(message));
    m_errorLabel->setVisible(true);
}

void NewWorkspaceDialog::resetForm()
{
    m_nameEdit->clear();
    m_errorLabel->setVisible(false);
    m_saveButton->setEnabled(false);
    m_loading = false;
}

void NewWorkspaceDialog::applyStyle(bool dark)
{
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString cardBg = ThemeColors::cardBackground(dark).name();
    const QString border = ThemeColors::border(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString textSecondary = ThemeColors::textSecondary(dark).name();
    const QString inputBg = ThemeColors::inputBackground(dark).name();
    const QString primary = ThemeColors::primary(dark).name();
    const QString primaryHover = ThemeColors::primaryHover(dark).name();

    setStyleSheet(QStringLiteral(
        "NewWorkspaceDialog {"
        "  background-color: %1;"
        "}"
        "QLabel {"
        "  color: %2;"
        "  font-size: 13px;"
        "}"
    ).arg(windowBg, textPrimary));

    m_nameEdit->setStyleSheet(QStringLiteral(
        "QLineEdit {"
        "  background-color: %1;"
        "  border: 1px solid %2;"
        "  border-radius: 8px;"
        "  padding: 6px 10px;"
        "  color: %3;"
        "  font-size: 13px;"
        "}"
        "QLineEdit::placeholder {"
        "  color: %4;"
        "}"
    ).arg(inputBg, border, textPrimary, textSecondary));

    m_errorLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: #EF4444; font-size: 12px; }"
    ));

    m_saveButton->setStyleSheet(QStringLiteral(
        "QPushButton {"
        "  background-color: %1;"
        "  color: #FFFFFF;"
        "  border: none;"
        "  border-radius: 8px;"
        "  padding: 8px 16px;"
        "  font-size: 13px;"
        "  font-weight: 500;"
        "}"
        "QPushButton:hover:!disabled {"
        "  background-color: %2;"
        "}"
        "QPushButton:disabled {"
        "  background-color: %3;"
        "  color: %4;"
        "}"
    ).arg(primary, primaryHover, border, textSecondary));

    if (QPushButton *cancelButton = m_buttonBox->button(QDialogButtonBox::Cancel)) {
        cancelButton->setStyleSheet(QStringLiteral(
            "QPushButton {"
            "  background-color: transparent;"
            "  color: %1;"
            "  border: 1px solid %2;"
            "  border-radius: 8px;"
            "  padding: 8px 16px;"
            "  font-size: 13px;"
            "}"
            "QPushButton:hover:!disabled {"
            "  background-color: %3;"
            "}"
        ).arg(textPrimary, border, ThemeColors::hoverBackground(dark).name()));
    }
}
