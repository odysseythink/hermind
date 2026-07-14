#include "ai_provider_settings_tab.h"

#include <QObject>

const QVector<AiProviderSettingsTab> &AiProviderSettingsTabs::all()
{
    static const QVector<AiProviderSettingsTab> tabs = {
        { QStringLiteral("llm-preference"),           QObject::tr("LLM Preference") },
        { QStringLiteral("vector-database"),          QObject::tr("Vector Database") },
        { QStringLiteral("embedding-preference"),     QObject::tr("Embedding Preference") },
        { QStringLiteral("text-splitter-preference"), QObject::tr("Text Splitting & Chunking") },
        { QStringLiteral("audio-preference"),         QObject::tr("Voice & Speech") },
        { QStringLiteral("transcription-preference"), QObject::tr("Transcription Model") },
        { QStringLiteral("model-routers"),            QObject::tr("Model Routers") },
    };
    return tabs;
}

int AiProviderSettingsTabs::indexOf(const QString &id)
{
    const auto &tabs = all();
    for (int i = 0; i < tabs.size(); ++i) {
        if (tabs.at(i).id == id)
            return i;
    }
    return -1;
}

QString AiProviderSettingsTabs::titleOf(const QString &id)
{
    const int idx = indexOf(id);
    if (idx < 0)
        return QString();
    return all().at(idx).title;
}
