#ifndef TST_AGENT_CONFIG_TAB_H
#define TST_AGENT_CONFIG_TAB_H

#include <QObject>

class TestAgentConfigTab : public QObject
{
    Q_OBJECT
private slots:
    void loadWorkspaceSetsProviderAndModel();
    void changingProviderEmitsDirty();
    void localProviderShowsPerformanceWarning();
    void unsupportedOpenAiModelShowsWarning();
    void buildUpdatePayloadContainsAgentFields();
    void agentSkillsButtonVisibleWhenClean();
    void agentSkillsButtonHiddenWhenDirty();
    void agentSkillsButtonEmitsNavigationRequest();
};

#endif // TST_AGENT_CONFIG_TAB_H
