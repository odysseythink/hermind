#include "agent_event_handler.h"

#include <QJsonArray>
#include <QUuid>

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
    m_bufferedCitations.clear();
    emit messagesChanged();
}

void AgentEventHandler::handleEvent(const HermindAgentEvent &event)
{
    const QString type = event.type();

    // No "type" field: generic message frame from the backend conversation
    // bridge (bridge.go OnMessage) — this is the agent's main text path.
    if (type.isEmpty()) {
        const QString content = event.content();
        if (content.isEmpty())
            return;
        appendStreamMessage(event.uuid(), content);
        return;
    }

    if (type == QLatin1String("reportStreamEvent")) {
        handleStreamEvent(event.contentObject());
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

// Mirrors the dispatch table in frontend/src/utils/chat/agent.js for
// reportStreamEvent frames whose content is an object.
void AgentEventHandler::handleStreamEvent(const QJsonObject &content)
{
    const QString subType = content.value(QLatin1String("type")).toString();
    const QString uuid = content.value(QLatin1String("uuid")).toString();
    if (subType.isEmpty())
        return;

    if (subType == QLatin1String("removeStatusResponse")) {
        for (int i = 0; i < m_messages.size(); ++i) {
            if (m_messages[i].uuid() == uuid) {
                m_messages.removeAt(i);
                emit messagesChanged();
                return;
            }
        }
        return;
    }

    if (subType == QLatin1String("citations")) {
        const QJsonArray citations = content.value(QLatin1String("citations")).toArray();
        if (citations.isEmpty())
            return;
        bool attached = false;
        for (HermindChatMessage &msg : m_messages) {
            if (msg.uuid() == uuid) {
                msg.appendSources(citations);
                attached = true;
                break;
            }
        }
        if (!attached && !uuid.isEmpty())
            m_bufferedCitations[uuid] = citations;
        if (attached)
            emit messagesChanged();
        emit citationsReceived(uuid, citations);
        return;
    }

    const QString text = content.value(QLatin1String("content")).toString();

    if (subType == QLatin1String("toolCallInvocation")) {
        // Accumulated tool-call content replaces the existing message entirely.
        for (HermindChatMessage &msg : m_messages) {
            if (msg.uuid() == uuid) {
                msg.setContent(text);
                emit messagesChanged();
                return;
            }
        }
        appendStreamMessage(uuid, text);
        return;
    }

    if (subType == QLatin1String("textResponseChunk")) {
        for (HermindChatMessage &msg : m_messages) {
            if (msg.uuid() == uuid) {
                msg.appendContent(text);
                emit messagesChanged();
                return;
            }
        }
        // Ignore a whitespace-only first chunk (some providers emit \n first).
        if (text.trimmed().isEmpty())
            return;
        appendStreamMessage(uuid, text);
        return;
    }

    // fullTextResponse and any generic stream sub-type: append to a known
    // message or create a new closed assistant message.
    if (text.isEmpty())
        return;
    if (subType != QLatin1String("fullTextResponse")) {
        for (HermindChatMessage &msg : m_messages) {
            if (msg.uuid() == uuid) {
                msg.appendContent(text);
                emit messagesChanged();
                return;
            }
        }
    }
    appendStreamMessage(uuid, text);
}

void AgentEventHandler::appendStreamMessage(const QString &uuid, const QString &content)
{
    HermindChatMessage msg;
    msg.setUuid(uuid.isEmpty() ? QUuid::createUuid().toString(QUuid::WithoutBraces) : uuid);
    msg.setRole(HermindChatMessage::Assistant);
    msg.setContent(content);
    msg.setClosed(true);
    msg.setSources(takeBufferedCitations(msg.uuid()));
    m_messages.append(msg);
    emit messagesChanged();
}

QJsonArray AgentEventHandler::takeBufferedCitations(const QString &uuid)
{
    if (uuid.isEmpty() || !m_bufferedCitations.contains(uuid))
        return {};
    return m_bufferedCitations.take(uuid);
}
