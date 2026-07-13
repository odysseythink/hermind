#include "hermind_workspace_user.h"

HermindWorkspaceUser HermindWorkspaceUser::fromJson(const QJsonObject &obj)
{
    HermindWorkspaceUser u;
    u.m_userId = obj.value(QStringLiteral("userId")).toInt();
    u.m_username = obj.value(QStringLiteral("username")).toString();
    if (obj.contains(QStringLiteral("role")) && !obj.value(QStringLiteral("role")).isNull())
        u.m_role = obj.value(QStringLiteral("role")).toString();
    u.m_lastUpdatedAt = QDateTime::fromString(obj.value(QStringLiteral("lastUpdatedAt")).toString(), Qt::ISODate);
    return u;
}

int HermindWorkspaceUser::userId() const { return m_userId; }
QString HermindWorkspaceUser::username() const { return m_username; }
QString HermindWorkspaceUser::role() const { return m_role; }
QDateTime HermindWorkspaceUser::lastUpdatedAt() const { return m_lastUpdatedAt; }
