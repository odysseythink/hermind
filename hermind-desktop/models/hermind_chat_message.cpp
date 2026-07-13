#include "hermind_chat_message.h"

HermindChatMessage HermindChatMessage::fromJson(const QJsonObject &obj)
{
    HermindChatMessage msg;
    msg.m_id = obj.value("id").toInt();
    msg.m_uuid = obj.value("uuid").toString();
    msg.m_role = roleFromString(obj.value("role").toString());
    msg.m_user = obj.value("user").toString();
    msg.m_content = obj.value("content").toString();
    msg.m_sentAt = QDateTime::fromSecsSinceEpoch(obj.value("sentAt").toVariant().toLongLong());
    msg.m_createdAt = QDateTime::fromSecsSinceEpoch(obj.value("createdAt").toVariant().toLongLong());
    msg.m_feedbackScore = obj.value("feedbackScore").toInt();
    msg.m_chatMode = obj.value("chatMode").toString();
    msg.m_closed = obj.value("close").toBool();
    msg.m_sources = obj.value("sources").toArray();
    msg.m_metrics = obj.value("metrics").toObject();
    return msg;
}

HermindChatMessage::Role HermindChatMessage::roleFromString(const QString &role)
{
    if (role == QLatin1String("user")) return User;
    if (role == QLatin1String("assistant")) return Assistant;
    if (role == QLatin1String("system")) return System;
    return Unknown;
}

int HermindChatMessage::id() const { return m_id; }
QString HermindChatMessage::uuid() const { return m_uuid; }
HermindChatMessage::Role HermindChatMessage::role() const { return m_role; }
QString HermindChatMessage::user() const { return m_user; }
QString HermindChatMessage::content() const { return m_content; }
QDateTime HermindChatMessage::sentAt() const { return m_sentAt; }
QDateTime HermindChatMessage::createdAt() const { return m_createdAt; }
int HermindChatMessage::feedbackScore() const { return m_feedbackScore; }
QString HermindChatMessage::chatMode() const { return m_chatMode; }
bool HermindChatMessage::isClosed() const { return m_closed; }
QJsonArray HermindChatMessage::sources() const { return m_sources; }
QJsonObject HermindChatMessage::metrics() const { return m_metrics; }

void HermindChatMessage::setUuid(const QString &uuid) { m_uuid = uuid; }
void HermindChatMessage::setRole(Role role) { m_role = role; }
void HermindChatMessage::setContent(const QString &content) { m_content = content; }
void HermindChatMessage::appendContent(const QString &chunk) { m_content += chunk; }
void HermindChatMessage::setClosed(bool closed) { m_closed = closed; }
void HermindChatMessage::setSources(const QJsonArray &sources) { m_sources = sources; }
void HermindChatMessage::appendSources(const QJsonArray &sources)
{
    for (const QJsonValue &v : sources)
        m_sources.append(v);
}
