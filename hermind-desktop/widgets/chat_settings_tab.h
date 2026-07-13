#ifndef CHAT_SETTINGS_TAB_H
#define CHAT_SETTINGS_TAB_H

#include <QWidget>

#include "api_response.h"
#include "hermind_workspace.h"

class HermindApiClient;
class QButtonGroup;
class QComboBox;
class QDoubleSpinBox;
class QLineEdit;
class QPushButton;
class QSpinBox;
class QTextEdit;

class ChatSettingsTab : public QWidget
{
    Q_OBJECT

public:
    explicit ChatSettingsTab(HermindApiClient *apiClient,
                             QWidget *parent = nullptr);

    void setWorkspaceSlug(const QString &slug);

signals:
    void workspaceUpdated(const HermindWorkspace &workspace);
    void hasChangesChanged(bool hasChanges);

private slots:
    void onWorkspaceLoaded(const HermindWorkspace &workspace,
                           const QString &message,
                           const ApiError &error);
    void onSystemKeysLoaded(const QJsonObject &settings,
                            const ApiError &error);
    void onDefaultPromptLoaded(const QString &prompt,
                               const ApiError &error);
    void onCustomModelsLoaded(const QStringList &models,
                              const ApiError &error);
    void onWorkspaceUpdated(const HermindWorkspace &workspace,
                            const QString &message,
                            const ApiError &error);

    void onProviderChanged(int index);
    void onModeChanged();
    void onFieldEdited();
    void onSaveClicked();
    void onResetPromptClicked();
    void applyStyle();

private:
    void buildUi();
    void setLoading(bool loading);
    void loadWorkspace();
    void loadCustomModels(const QString &provider);
    void updateModelSelector();
    void updateHasChanges();
    QJsonObject collectFields() const;
    QString currentProvider() const;
    QString currentModel() const;

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    HermindWorkspace m_workspace;
    QString m_systemLlmProvider;
    QString m_defaultSystemPrompt;
    QStringList m_pendingCustomModels;
    bool m_hasChanges = false;
    bool m_loadingWorkspace = false;

    QComboBox *m_providerCombo = nullptr;
    QComboBox *m_modelCombo = nullptr;
    QLineEdit *m_modelLineEdit = nullptr;
    QButtonGroup *m_modeGroup = nullptr;
    QSpinBox *m_historySpin = nullptr;
    QDoubleSpinBox *m_tempSpin = nullptr;
    QTextEdit *m_promptEdit = nullptr;
    QTextEdit *m_refusalEdit = nullptr;
    QPushButton *m_resetPromptButton = nullptr;
    QPushButton *m_saveButton = nullptr;
};

#endif // CHAT_SETTINGS_TAB_H
