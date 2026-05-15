#ifndef SETTINGSEDITOR_H
#define SETTINGSEDITOR_H

#include <QDialog>
#include <QJsonObject>
#include <QHash>

class HermindClient;
class QListWidget;
class QStackedWidget;
class QPushButton;
class ConfigFormEngine;

class SettingsEditor : public QDialog
{
    Q_OBJECT
public:
    explicit SettingsEditor(HermindClient *client, QWidget *parent = nullptr);

private slots:
    void loadSchema();
    void loadValues();
    void onGroupSelected(int index);
    void onSaveClicked();
    void onCancelClicked();
    void onDirtyChanged(bool dirty);

private:
    void setupUI();
    void updateSaveButton();

    HermindClient *m_client;
    QListWidget *m_sidebar;
    QStackedWidget *m_panelStack;
    QHash<QString, ConfigFormEngine*> m_forms;
    QPushButton *m_saveBtn;
    QPushButton *m_cancelBtn;
    bool m_anyDirty;
};

#endif
