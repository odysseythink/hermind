#include "llm_model_selector.h"
#include "llm_provider_info.h"

QStringList LlmModelSelector::modelsForProvider(const QString &provider,
                                                const QStringList &customModels)
{
    const LlmProviderInfo *info = LlmProviderInfo::byId(provider);
    if (!info || !info->supportsModelSelection)
        return QStringList();

    QStringList result = info->defaultModels;
    for (const QString &m : customModels) {
        if (!result.contains(m))
            result.append(m);
    }
    return result;
}

bool LlmModelSelector::supportsModelSelection(const QString &provider)
{
    const LlmProviderInfo *info = LlmProviderInfo::byId(provider);
    return info && info->supportsModelSelection;
}

bool LlmModelSelector::isManualModelInput(const QString &provider)
{
    const LlmProviderInfo *info = LlmProviderInfo::byId(provider);
    return info && !info->supportsModelSelection && info->id != QStringLiteral("default");
}

double LlmModelSelector::recommendedTemperature(const QString &provider)
{
    if (provider == QStringLiteral("mistral"))
        return 0.0;
    return 0.7;
}
