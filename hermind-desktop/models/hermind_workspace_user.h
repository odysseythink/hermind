#ifndef HERMIND_WORKSPACE_USER_H
#define HERMIND_WORKSPACE_USER_H

#include <QString>
#include <QJsonObject>
#include <QDateTime>

class HermindWorkspaceUser
{
public:
    HermindWorkspaceUser() = default;
    static HermindWorkspaceUser fromJson(const QJsonObject &obj);

    int userId() const;
    QString username() const;
    QString role() const;
    QDateTime lastUpdatedAt() const;

private:
    int m_userId = 0;
    QString m_username;
    QString m_role = QStringLiteral("default");
    QDateTime m_lastUpdatedAt;
};

#endif // HERMIND_WORKSPACE_USER_H
