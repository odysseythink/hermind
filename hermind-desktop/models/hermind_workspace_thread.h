#ifndef HERMIND_WORKSPACE_THREAD_H
#define HERMIND_WORKSPACE_THREAD_H

#include <QString>
#include <QJsonObject>
#include <QDateTime>
#include <optional>

class HermindWorkspaceThread
{
public:
    HermindWorkspaceThread() = default;

    static HermindWorkspaceThread fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    int id() const;
    QString name() const;
    QString slug() const;
    int workspaceId() const;
    std::optional<int> userId() const;
    std::optional<int> parentThreadId() const;
    QDateTime createdAt() const;
    QDateTime lastUpdatedAt() const;

private:
    static std::optional<int> optionalInt(const QJsonObject &obj, const char *key);

    int m_id = 0;
    QString m_name;
    QString m_slug;
    int m_workspaceId = 0;
    std::optional<int> m_userId;
    std::optional<int> m_parentThreadId;
    QDateTime m_createdAt;
    QDateTime m_lastUpdatedAt;
};

#endif // HERMIND_WORKSPACE_THREAD_H
