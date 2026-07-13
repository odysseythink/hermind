#ifndef TST_LLM_PROVIDER_INFO_H
#define TST_LLM_PROVIDER_INFO_H

#include <QObject>

class TestLlmProviderInfo : public QObject
{
    Q_OBJECT

private slots:
    void cohereHasDefaultModels();
    void openAiDefaultModelsEmpty();
    void defaultProviderHasNoModels();
};

#endif // TST_LLM_PROVIDER_INFO_H
