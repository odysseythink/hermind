#ifndef HERMIND_CHAT_MESSAGE_H
#define HERMIND_CHAT_MESSAGE_H

#include <QString>
#include <QDateTime>
#include <QJsonObject>
#include <QJsonArray>

class HermindChatMessage
{
public:
    enum Role {
        User,
        Assistant,
        System,
        Unknown
    };

    HermindChatMessage() = default;
    static HermindChatMessage fromJson(const QJsonObject &obj);

    int id() const;
    QString uuid() const;
    Role role() const;
    QString user() const;
    QString content() const;
    QDateTime sentAt() const;
    QDateTime createdAt() const;
    int feedbackScore() const;
    QString chatMode() const;
    bool isClosed() const;
    QJsonArray sources() const;
    QJsonObject metrics() const;

    void setUuid(const QString &uuid);
    void setRole(Role role);
    void setContent(const QString &content);
    void appendContent(const QString &chunk);
    void setClosed(bool closed);
    void setSources(const QJsonArray &sources);
    void appendSources(const QJsonArray &sources);

private:
    static Role roleFromString(const QString &role);

    int m_id = 0;
    QString m_uuid;
    Role m_role = Unknown;
    QString m_user;
    QString m_content;
    QDateTime m_sentAt;
    QDateTime m_createdAt;
    int m_feedbackScore = 0;
    QString m_chatMode;
    bool m_closed = false;
    QJsonArray m_sources;
    QJsonObject m_metrics;
};

#endif // HERMIND_CHAT_MESSAGE_H
