#ifndef TST_LLM_MODEL_SELECTOR_H
#define TST_LLM_MODEL_SELECTOR_H

#include <QObject>

class TestLlmModelSelector : public QObject
{
    Q_OBJECT

private slots:
    void defaultProviderReturnsEmptyModels();
    void azureRequiresManualInput();
    void openAiUsesDiscoveredModels();
    void cohereFallsBackToDefaults();
    void duplicatesAreRemoved();
    void recommendedTemperature();
};

#endif // TST_LLM_MODEL_SELECTOR_H
