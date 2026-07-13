#include "agent_event_handler.h"

AgentEventHandler::AgentEventHandler(QObject *parent)
    : QObject(parent)
{
}

const QVector<HermindChatMessage> &AgentEventHandler::messages() const
{
    return m_messages;
}

void AgentEventHandler::clear()
{
    m_messages.clear();
    emit messagesChanged();
}

void AgentEventHandler::handleEvent(const HermindAgentEvent &event)
{
    const QString type = event.type();

    if (type == QLatin1String("reportStreamEvent")) {
        HermindChatMessage msg;
        msg.setUuid(QString::number(qHash(event.content()))); // 简易 UUID；后续用真实 id
        msg.setRole(HermindChatMessage::Assistant);
        msg.setContent(event.content());
        msg.setClosed(true);
        m_messages.append(msg);
        emit messagesChanged();
    } else if (type == QLatin1String("statusResponse")) {
        emit statusReceived(event.content());
    } else if (type == QLatin1String("fileDownloadCard")) {
        emit downloadCardReceived(event.payload().toObject());
    } else if (type == QLatin1String("rechartVisualize")) {
        emit visualizeChartReceived(event.payload().toObject());
    } else if (type == QLatin1String("toolApprovalRequest")) {
        emit toolApprovalRequested(event.requestId(), event.skillName(), event.description());
    } else if (type == QLatin1String("clarificationRequest")) {
        emit clarificationRequested(event.question());
    } else if (type == QLatin1String("wssFailure")) {
        emit errorReceived(event.content());
    } else if (type == QLatin1String("action")) {
        const QString action = event.content();
        if (action == QLatin1String("rename_thread"))
            emit threadRenameRequested(event.payload().toObject().value("name").toString());
    }
}
