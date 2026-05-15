#include "settingsdialog.h"
#include "httplib.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QFormLayout>
#include <QComboBox>
#include <QLineEdit>
#include <QPushButton>
#include <QLabel>
#include <QJsonObject>
#include <QMessageBox>

SettingsDialog::SettingsDialog(HermindClient *client, QWidget *parent)
    : QDialog(parent),
      m_client(client)
{
    setWindowTitle("Settings");
    resize(400, 200);

    QFormLayout *formLayout = new QFormLayout();

    m_providerCombo = new QComboBox(this);
    m_providerCombo->addItem("OpenAI");
    m_providerCombo->addItem("Anthropic");
    m_providerCombo->addItem("Google");
    formLayout->addRow("Provider:", m_providerCombo);

    m_apiKeyEdit = new QLineEdit(this);
    m_apiKeyEdit->setEchoMode(QLineEdit::Password);
    formLayout->addRow("API Key:", m_apiKeyEdit);

    m_themeCombo = new QComboBox(this);
    m_themeCombo->addItem("Dark");
    m_themeCombo->addItem("Light");
    formLayout->addRow("Theme:", m_themeCombo);

    QPushButton *saveButton = new QPushButton("Save", this);
    connect(saveButton, &QPushButton::clicked, this, &SettingsDialog::onSaveClicked);

    QHBoxLayout *buttonLayout = new QHBoxLayout();
    buttonLayout->addStretch(1);
    buttonLayout->addWidget(saveButton);

    QVBoxLayout *mainLayout = new QVBoxLayout(this);
    mainLayout->addLayout(formLayout);
    mainLayout->addStretch(1);
    mainLayout->addLayout(buttonLayout);
}

void SettingsDialog::onSaveClicked()
{
    if (!m_client) {
        QMessageBox::warning(this, "Error", "No client connection available.");
        return;
    }

    QJsonObject body;
    body["provider"] = m_providerCombo->currentText();
    body["api_key"] = m_apiKeyEdit->text();
    body["theme"] = m_themeCombo->currentText().toLower();

    m_client->post("/api/config", body,
                   [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            QMessageBox::warning(this, "Error", "Failed to save settings: " + error);
        } else {
            QMessageBox::information(this, "Success", "Settings saved successfully.");
            accept();
        }
    });
}
