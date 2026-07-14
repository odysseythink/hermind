/*****************************************************************************
 * AgentConfigTab — workspace-level default agent provider/model selection.
 *
 * This tab is part of the workspace settings dialog. It allows the user to
 * pick a default LLM provider and model for agentic chat in this workspace,
 * overriding the global server defaults. Changes are submitted by clicking the
 * save button, which calls the workspace update API.
 *****************************************************************************/
#ifndef AGENT_CONFIG_TAB_H
#define AGENT_CONFIG_TAB_H

#include <QWidget>
#include <memory>

#include "agent_config_state.h"
#include "hermind_workspace.h"

class ApiError;

class HermindApiClient;
class QComboBox;
class QLabel;
class QPushButton;
class QVBoxLayout;
struct LlmProviderInfo;

class AgentConfigTab : public QWidget
{
    Q_OBJECT

public:
    explicit AgentConfigTab(HermindApiClient *apiClient,
                            QWidget *parent = nullptr);

    QString workspaceSlug() const;

public slots:
    void setWorkspaceSlug(const QString &slug);
    void reload();

    // Direct load path, mainly used by tests.
    void loadFromWorkspace(const HermindWorkspace &workspace);

    bool isDirty() const;
    QJsonObject buildUpdatePayload() const;

signals:
    void configLoaded(bool success);
    void workspaceUpdated(const HermindWorkspace &workspace);
    void dirtyChanged(bool dirty);
    void performanceWarningChanged(bool warning);
    void agentSkillsRequested();

private slots:
    void onWorkspaceLoaded(const HermindWorkspace &workspace,
                           const QString &message,
                           const ApiError &error);
    void onModelsLoaded(const QStringList &models,
                        const ApiError &error);
    void onUpdateFinished(const HermindWorkspace &workspace,
                          const QString &message,
                          const ApiError &error);
    void onProviderChanged(int index);
    void onModelChanged(const QString &model);
    void onSaveClicked();
    void applyTheme();

private:
    void buildUi();
    void loadWorkspace();
    void loadCustomModels();
    void refreshModelList();
    void updateWarning();
    void updateSaveButton();
    void updateSkillsButton();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    AgentConfigState m_state;
    bool m_blockSignals = false;

    QVBoxLayout *m_layout = nullptr;
    QComboBox *m_providerCombo = nullptr;
    QComboBox *m_modelCombo = nullptr;
    QLabel *m_warningLabel = nullptr;
    QPushButton *m_saveButton = nullptr;
    QWidget *m_skillsRow = nullptr;
    QPushButton *m_agentSkillsButton = nullptr;
};

#endif // AGENT_CONFIG_TAB_H
