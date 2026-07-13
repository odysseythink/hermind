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

private:
    QVector<HermindChatMessage> m_messages;
};

#endif // AGENT_EVENT_HANDLER_H
