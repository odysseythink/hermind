/*****************************************************************************
 * AgentConfigTab — workspace-level default agent provider/model selection.
 *
 * This tab is part of the workspace settings dialog. It allows the user to
 * pick a default LLM provider and model for agentic chat in this workspace,
 * overriding the global server defaults. Changes are only submitted when the
 * user explicitly saves the workspace settings.
 *****************************************************************************/
#ifndef AGENT_CONFIG_TAB_H
#define AGENT_CONFIG_TAB_H

#include <QWidget>
#include <memory>

#include "agent_config_state.h"
#include "hermind_workspace.h"

class QComboBox;
class QLabel;
class QVBoxLayout;
struct LlmProviderInfo;

class AgentConfigTab : public QWidget
{
    Q_OBJECT

public:
    explicit AgentConfigTab(QWidget *parent = nullptr);
    ~AgentConfigTab() override;

    void loadFromWorkspace(const HermindWorkspace &workspace);

    // Returns true if the current selection differs from the loaded workspace.
    bool isDirty() const;

    // JSON payload fragment for the workspace update API.
    QJsonObject buildUpdatePayload() const;

    // Reset the form to the last loaded workspace values.
    void reset();

signals:
    // Emitted whenever the dirty state changes.
    void dirtyChanged(bool dirty);

    // Emitted when a model is selected that is not on the recommended list.
    void performanceWarningChanged(bool warning);

private slots:
    void onProviderChanged(int index);
    void onModelChanged(const QString &model);

private:
    void buildUi();
    void applyTheme();
    void refreshModelList();
    void updateWarning();

    AgentConfigState m_state;
    bool m_blockSignals = false;

    QVBoxLayout *m_layout = nullptr;
    QComboBox *m_providerCombo = nullptr;
    QComboBox *m_modelSelector = nullptr;
    QLabel *m_warningLabel = nullptr;
};

#endif // AGENT_CONFIG_TAB_H
