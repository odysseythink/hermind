#ifndef SETTINGSDIALOG_H
#define SETTINGSDIALOG_H

#include <QDialog>

class QComboBox;
class QLineEdit;
class HermindClient;

class SettingsDialog : public QDialog
{
    Q_OBJECT
public:
    explicit SettingsDialog(HermindClient *client, QWidget *parent = nullptr);

private slots:
    void onSaveClicked();

private:
    HermindClient *m_client;
    QComboBox *m_providerCombo;
    QLineEdit *m_apiKeyEdit;
    QComboBox *m_themeCombo;
};

#endif
