#include "workspace_settings_tab.h"

#include <QObject>

const QVector<WorkspaceSettingsTab> &WorkspaceSettingsTabs::all()
{
    static const QVector<WorkspaceSettingsTab> tabs = {
        { QStringLiteral("general-appearance"), QObject::tr("General Appearance") },
        { QStringLiteral("chat"),                 QObject::tr("Chat Settings") },
        { QStringLiteral("vector-database"),      QObject::tr("Vector Database") },
        { QStringLiteral("members"),              QObject::tr("Members") },
        { QStringLiteral("agent-config"),         QObject::tr("Agent Configuration") },
    };
    return tabs;
}

int WorkspaceSettingsTabs::indexOf(const QString &id)
{
    const auto &tabs = all();
    for (int i = 0; i < tabs.size(); ++i) {
        if (tabs.at(i).id == id)
            return i;
    }
    return -1;
}

QString WorkspaceSettingsTabs::titleOf(const QString &id)
{
    const int idx = indexOf(id);
    if (idx < 0)
        return QString();
    return all().at(idx).title;
}
