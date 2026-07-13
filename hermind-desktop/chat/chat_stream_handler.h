#ifndef CHAT_STREAM_HANDLER_H
#define CHAT_STREAM_HANDLER_H

#include <QObject>
#include <QVector>
#include <QHash>
#include "hermind_chat_message.h"
#include "hermind_stream_chat_response.h"

class ChatStreamHandler : public QObject
{
    Q_OBJECT
public:
    explicit ChatStreamHandler(QObject *parent = nullptr);

    const QVector<HermindChatMessage> &messages() const;
    void setMessages(const QVector<HermindChatMessage> &messages);
    void clear();

    void handleResponse(const HermindStreamChatResponse &response);

    // Close the last message if it is still open (e.g. after a user abort,
    // where no finalizing frame ever arrives from the server).
    void closeLastMessage();

signals:
    void messagesChanged();
    void streamFinished();
    void errorReceived(const QString &message);
    void agentWebSocketRequested(const QString &socketId, const QString &token);
    void sourcesReceived(const QString &uuid, const QJsonArray &sources);

private:
    int findMessageIndexByUuid(const QString &uuid) const;
    void appendOrUpdateAssistant(const QString &uuid, const QString &text, bool close);

    QVector<HermindChatMessage> m_messages;
    QHash<QString, int> m_uuidToIndex;
};

#endif // CHAT_STREAM_HANDLER_H
