#include "hermind_workspace_thread.h"

std::optional<int> HermindWorkspaceThread::optionalInt(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toInt();
}

HermindWorkspaceThread HermindWorkspaceThread::fromJson(const QJsonObject &obj)
{
    HermindWorkspaceThread t;
    t.m_id = obj.value(QStringLiteral("id")).toInt();
    t.m_name = obj.value(QStringLiteral("name")).toString();
    t.m_slug = obj.value(QStringLiteral("slug")).toString();
    t.m_workspaceId = obj.value(QStringLiteral("workspaceId")).toInt();
    t.m_userId = optionalInt(obj, "userId");
    t.m_parentThreadId = optionalInt(obj, "parentThreadId");
    t.m_createdAt = QDateTime::fromString(obj.value(QStringLiteral("createdAt")).toString(), Qt::ISODate);
    t.m_lastUpdatedAt = QDateTime::fromString(obj.value(QStringLiteral("lastUpdatedAt")).toString(), Qt::ISODate);
    return t;
}

QJsonObject HermindWorkspaceThread::toJson() const
{
    QJsonObject obj;
    obj.insert(QStringLiteral("id"), m_id);
    obj.insert(QStringLiteral("name"), m_name);
    obj.insert(QStringLiteral("slug"), m_slug);
    obj.insert(QStringLiteral("workspaceId"), m_workspaceId);
    obj.insert(QStringLiteral("userId"), m_userId.has_value() ? QJsonValue(m_userId.value()) : QJsonValue::Null);
    obj.insert(QStringLiteral("parentThreadId"), m_parentThreadId.has_value() ? QJsonValue(m_parentThreadId.value()) : QJsonValue::Null);
    obj.insert(QStringLiteral("createdAt"), m_createdAt.toString(Qt::ISODate));
    obj.insert(QStringLiteral("lastUpdatedAt"), m_lastUpdatedAt.toString(Qt::ISODate));
    return obj;
}

int HermindWorkspaceThread::id() const { return m_id; }
QString HermindWorkspaceThread::name() const { return m_name; }
QString HermindWorkspaceThread::slug() const { return m_slug; }
int HermindWorkspaceThread::workspaceId() const { return m_workspaceId; }
std::optional<int> HermindWorkspaceThread::userId() const { return m_userId; }
std::optional<int> HermindWorkspaceThread::parentThreadId() const { return m_parentThreadId; }
QDateTime HermindWorkspaceThread::createdAt() const { return m_createdAt; }
QDateTime HermindWorkspaceThread::lastUpdatedAt() const { return m_lastUpdatedAt; }
