#ifndef AGENT_EVENT_HANDLER_H
#define AGENT_EVENT_HANDLER_H

#include <QObject>
#include <QVector>
#include "hermind_chat_message.h"
#include "hermind_agent_event.h"

class AgentEventHandler : public QObject
{
    Q_OBJECT
public:
    explicit AgentEventHandler(QObject *parent = nullptr);

    const QVector<HermindChatMessage> &messages() const;
    void clear();

    void handleEvent(const HermindAgentEvent &event);

signals:
    void messagesChanged();
    void statusReceived(const QString &status);
    void downloadCardReceived(const QJsonObject &payload);
    void visualizeChartReceived(const QJsonObject &payload);
    void toolApprovalRequested(const QString &requestId, const QString &skillName, const QString &description);
    void clarificationRequested(const QString &question);
    void errorReceived(const QString &error);
    void threadRenameRequested(const QString &newName);
    void citationsReceived(const QString &uuid, const QJsonArray &citations);

private:
    void handleStreamEvent(const QJsonObject &content);
    void appendStreamMessage(const QString &uuid, const QString &content);
    QJsonArray takeBufferedCitations(const QString &uuid);

    QVector<HermindChatMessage> m_messages;
    // Citations can arrive before their message exists (empty->chat remount);
    // buffer by uuid and attach when the message appears (frontend agent.js).
    QHash<QString, QJsonArray> m_bufferedCitations;
};

#endif // AGENT_EVENT_HANDLER_H
