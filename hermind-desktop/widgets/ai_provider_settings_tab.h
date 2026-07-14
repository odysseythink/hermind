#ifndef AI_PROVIDER_SETTINGS_TAB_H
#define AI_PROVIDER_SETTINGS_TAB_H

#include <QString>
#include <QVector>

struct AiProviderSettingsTab {
    QString id;
    QString title;
};

class AiProviderSettingsTabs
{
public:
    static const QVector<AiProviderSettingsTab> &all();
    static int indexOf(const QString &id);
    static QString titleOf(const QString &id);
};

#endif // AI_PROVIDER_SETTINGS_TAB_H
