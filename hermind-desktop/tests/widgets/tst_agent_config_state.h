#ifndef TST_AGENT_CONFIG_STATE_H
#define TST_AGENT_CONFIG_STATE_H

#include <QObject>

class TestAgentConfigState : public QObject
{
    Q_OBJECT
private slots:
    void defaultProviderIsNotDirty();
    void selectingProviderMakesDirty();
    void localProviderShowsPerformanceWarning();
    void unsupportedOpenAiModelsAreRejected();
    void payloadForDefaultProviderEmptiesAgentProvider();
    void payloadForSelectedProvider();
    void resetRestoresOriginalValues();
};

#endif // TST_AGENT_CONFIG_STATE_H
