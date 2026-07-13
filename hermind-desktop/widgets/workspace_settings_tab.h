#ifndef WORKSPACE_SETTINGS_TAB_H
#define WORKSPACE_SETTINGS_TAB_H

#include <QString>
#include <QVector>

struct WorkspaceSettingsTab {
    QString id;
    QString title;
};

class WorkspaceSettingsTabs
{
public:
    static const QVector<WorkspaceSettingsTab> &all();
    static int indexOf(const QString &id);
    static QString titleOf(const QString &id);
};

#endif // WORKSPACE_SETTINGS_TAB_H
