#ifndef LLM_MODEL_SELECTOR_H
#define LLM_MODEL_SELECTOR_H

#include <QString>
#include <QStringList>

class LlmModelSelector
{
public:
    static QStringList modelsForProvider(const QString &provider,
                                         const QStringList &customModels);
    static bool supportsModelSelection(const QString &provider);
    static bool isManualModelInput(const QString &provider);
    static double recommendedTemperature(const QString &provider);
};

#endif // LLM_MODEL_SELECTOR_H
