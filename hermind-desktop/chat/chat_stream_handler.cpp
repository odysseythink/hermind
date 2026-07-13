#include "chat_stream_handler.h"

ChatStreamHandler::ChatStreamHandler(QObject *parent)
    : QObject(parent)
{
}

const QVector<HermindChatMessage> &ChatStreamHandler::messages() const
{
    return m_messages;
}

void ChatStreamHandler::setMessages(const QVector<HermindChatMessage> &messages)
{
    m_messages = messages;
    m_uuidToIndex.clear();
    for (int i = 0; i < m_messages.size(); ++i)
        m_uuidToIndex.insert(m_messages.at(i).uuid(), i);
    emit messagesChanged();
}

void ChatStreamHandler::clear()
{
    m_messages.clear();
    m_uuidToIndex.clear();
    emit messagesChanged();
}

int ChatStreamHandler::findMessageIndexByUuid(const QString &uuid) const
{
    if (uuid.isEmpty())
        return -1;
    return m_uuidToIndex.value(uuid, -1);
}

void ChatStreamHandler::appendOrUpdateAssistant(const QString &uuid,
                                                const QString &text,
                                                bool close)
{
    int idx = findMessageIndexByUuid(uuid);
    if (idx < 0) {
        HermindChatMessage msg;
        msg.setUuid(uuid);
        msg.setRole(HermindChatMessage::Assistant);
        msg.setContent(text);
        msg.setClosed(close);
        m_messages.append(msg);
        m_uuidToIndex.insert(uuid, m_messages.size() - 1);
    } else {
        m_messages[idx].appendContent(text);
        m_messages[idx].setClosed(close);
    }
    emit messagesChanged();
}

void ChatStreamHandler::closeLastMessage()
{
    if (!m_messages.isEmpty() && !m_messages.last().isClosed()) {
        m_messages.last().setClosed(true);
        emit messagesChanged();
    }
}

void ChatStreamHandler::handleResponse(const HermindStreamChatResponse &response)
{
    const QString type = response.type();
    const QString uuid = response.uuid();

    const QJsonArray sources = response.sources();
    if (!sources.isEmpty()) {
        int idx = findMessageIndexByUuid(uuid);
        if (idx < 0 && !m_messages.isEmpty())
            idx = m_messages.size() - 1;
        if (idx >= 0) {
            m_messages[idx].appendSources(sources);
            emit messagesChanged();
        }
        emit sourcesReceived(uuid, sources);
    }

    if (type == QLatin1String("textResponse")) {
        if (const auto text = response.textResponse())
            appendOrUpdateAssistant(uuid, *text, response.close());
    } else if (type == QLatin1String("textResponseChunk")) {
        if (const auto text = response.textResponse())
            appendOrUpdateAssistant(uuid, *text, response.close());
    } else if (type == QLatin1String("finalizeResponseStream")) {
        appendOrUpdateAssistant(uuid, QString(), true);
    } else if (type == QLatin1String("abort")) {
        appendOrUpdateAssistant(uuid.isEmpty() ? m_messages.isEmpty() ? QString() : m_messages.last().uuid() : uuid,
                                QString(), true);
    } else if (type == QLatin1String("statusResponse")) {
        // 本阶段仅记录，不渲染状态卡片
    } else if (type == QLatin1String("stopGeneration")) {
        if (!m_messages.isEmpty())
            m_messages.last().setClosed(true);
        emit streamFinished();
    } else if (type == QLatin1String("modelRouteNotification")) {
        if (const auto routed = response.routedTo()) {
            if (!routed->isEmpty()) {
                // 可更新 UI 状态，本阶段仅透传
            }
        }
    } else if (type == QLatin1String("agentInitWebsocketConnection")) {
        const auto socketId = response.websocketUUID();
        const auto token = response.websocketToken();
        if (socketId && token)
            emit agentWebSocketRequested(*socketId, *token);
    } else if (type == QLatin1String("action")) {
        const auto action = response.action();
        if (action == QLatin1String("reset_chat")) {
            clear();
        } else if (action == QLatin1String("rename_thread")) {
            // 留给容器处理
        }
    }
}
