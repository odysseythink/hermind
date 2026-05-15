#include "settingseditor.h"
#include "httplib.h"
#include "configformengine.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QListWidget>
#include <QStackedWidget>
#include <QPushButton>
#include <QLabel>
#include <QMessageBox>
#include <QDebug>

SettingsEditor::SettingsEditor(HermindClient *client, QWidget *parent)
    : QDialog(parent),
      m_client(client),
      m_sidebar(new QListWidget(this)),
      m_panelStack(new QStackedWidget(this)),
      m_saveBtn(new QPushButton(QStringLiteral("Save"), this)),
      m_cancelBtn(new QPushButton(QStringLiteral("Cancel"), this)),
      m_anyDirty(false)
{
    setWindowTitle(QStringLiteral("Settings"));
    resize(800, 600);
    setupUI();
    loadSchema();
}

void SettingsEditor::setupUI()
{
    QHBoxLayout *mainLayout = new QHBoxLayout(this);
    mainLayout->setContentsMargins(0, 0, 0, 0);
    mainLayout->setSpacing(0);

    // Sidebar
    m_sidebar->setFixedWidth(180);
    m_sidebar->setStyleSheet(
        QStringLiteral(
            "QListWidget { background: #0f1012; border: none; outline: none; }"
            "QListWidget::item { color: #8a8680; padding: 10px 16px; border-left: 3px solid transparent; }"
            "QListWidget::item:selected { background: #1a1c20; color: #FFB800; border-left: 3px solid #FFB800; }"
            "QListWidget::item:hover { background: #14161a; }"
        )
    );
    connect(m_sidebar, &QListWidget::currentRowChanged,
            this, &SettingsEditor::onGroupSelected);

    // Right panel with stack + buttons
    QVBoxLayout *rightLayout = new QVBoxLayout();
    rightLayout->setContentsMargins(0, 0, 0, 0);
    rightLayout->setSpacing(0);
    rightLayout->addWidget(m_panelStack, 1);

    QHBoxLayout *btnLayout = new QHBoxLayout();
    btnLayout->setContentsMargins(16, 12, 16, 12);
    btnLayout->setSpacing(8);
    btnLayout->addStretch(1);
    btnLayout->addWidget(m_cancelBtn);
    btnLayout->addWidget(m_saveBtn);
    rightLayout->addLayout(btnLayout);

    mainLayout->addWidget(m_sidebar);
    mainLayout->addLayout(rightLayout, 1);

    connect(m_saveBtn, &QPushButton::clicked, this, &SettingsEditor::onSaveClicked);
    connect(m_cancelBtn, &QPushButton::clicked, this, &SettingsEditor::onCancelClicked);

    m_saveBtn->setEnabled(false);
    m_saveBtn->setCursor(Qt::PointingHandCursor);
    m_saveBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: #FFB800; color: #0a0b0d; padding: 6px 20px; "
            "border-radius: 4px; font-weight: 600; border: none; }"
            "QPushButton:hover { background: #FF8A00; }"
            "QPushButton:disabled { background: #2a2e36; color: #5a5e66; }"
        )
    );
    m_cancelBtn->setCursor(Qt::PointingHandCursor);
    m_cancelBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: transparent; color: #8a8680; padding: 6px 20px; "
            "border: 1px solid #2a2e36; border-radius: 4px; }"
            "QPushButton:hover { color: #e8e6e3; border-color: #3a3e46; }"
        )
    );
}

void SettingsEditor::loadSchema()
{
    if (!m_client)
        return;

    m_client->get(QStringLiteral("/api/config/schema"),
                  [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            QMessageBox::warning(this, QStringLiteral("Error"),
                                 QStringLiteral("Failed to load settings schema: ") + error);
            return;
        }

        QJsonObject groups = resp.value(QStringLiteral("groups")).toObject();
        if (groups.isEmpty()) {
            // Fallback: treat top-level keys as a single group if no groups field
            groups.insert(QStringLiteral("General"), resp);
        }

        for (auto it = groups.begin(); it != groups.end(); ++it) {
            QString groupName = it.key();
            QJsonObject groupSchema = it.value().toObject();

            ConfigFormEngine *form = new ConfigFormEngine(this);
            connect(form, &ConfigFormEngine::dirtyChanged,
                    this, &SettingsEditor::onDirtyChanged);
            form->buildForm(groupSchema);

            m_forms.insert(groupName, form);
            m_sidebar->addItem(groupName);
            m_panelStack->addWidget(form);
        }

        if (m_sidebar->count() > 0) {
            m_sidebar->setCurrentRow(0);
        }

        loadValues();
    });
}

void SettingsEditor::loadValues()
{
    if (!m_client)
        return;

    m_client->get(QStringLiteral("/api/config"),
                  [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            qWarning() << "Failed to load config values:" << error;
            return;
        }
        for (ConfigFormEngine *form : m_forms) {
            form->setValues(resp);
        }
        m_anyDirty = false;
        updateSaveButton();
    });
}

void SettingsEditor::onGroupSelected(int index)
{
    m_panelStack->setCurrentIndex(index);
}

void SettingsEditor::onSaveClicked()
{
    if (!m_client)
        return;

    QJsonObject allValues;
    for (ConfigFormEngine *form : m_forms) {
        QJsonObject groupValues = form->values();
        for (auto it = groupValues.begin(); it != groupValues.end(); ++it) {
            allValues[it.key()] = it.value();
        }
    }

    m_client->post(QStringLiteral("/api/config"), allValues,
                   [this](const QJsonObject &resp, const QString &error) {
        Q_UNUSED(resp)
        if (!error.isEmpty()) {
            QMessageBox::warning(this, QStringLiteral("Error"),
                                 QStringLiteral("Failed to save settings: ") + error);
            return;
        }
        for (ConfigFormEngine *form : m_forms) {
            form->setValues(form->values());
        }
        m_anyDirty = false;
        updateSaveButton();
        QMessageBox::information(this, QStringLiteral("Success"),
                                 QStringLiteral("Settings saved successfully."));
    });
}

void SettingsEditor::onCancelClicked()
{
    reject();
}

void SettingsEditor::onDirtyChanged(bool dirty)
{
    Q_UNUSED(dirty)
    bool any = false;
    for (ConfigFormEngine *form : m_forms) {
        if (form->isDirty()) {
            any = true;
            break;
        }
    }
    m_anyDirty = any;
    updateSaveButton();
}

void SettingsEditor::updateSaveButton()
{
    m_saveBtn->setEnabled(m_anyDirty);
}
