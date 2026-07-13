#ifndef LLM_PROVIDER_INFO_H
#define LLM_PROVIDER_INFO_H

#include <QString>
#include <QStringList>
#include <QVector>

struct LlmProviderInfo
{
    QString id;
    QString name;
    QString description;
    bool supportsModelSelection = true;
    QStringList defaultModels;

    static const QVector<LlmProviderInfo> &all();
    static const LlmProviderInfo *byId(const QString &id);
    static QStringList ids();
};

#endif // LLM_PROVIDER_INFO_H
