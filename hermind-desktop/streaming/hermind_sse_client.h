#ifndef HERMIND_SSE_CLIENT_H
#define HERMIND_SSE_CLIENT_H

#include <QObject>
#include <QUrl>
#include <QJsonObject>
#include <QNetworkReply>
#include <functional>

#include "api_response.h"
#include "hermind_stream_chat_response.h"

class QNetworkAccessManager;

class HermindSseClient : public QObject
{
    Q_OBJECT

public:
    using EventCallback = std::function<void(const HermindStreamChatResponse &response)>;
    using ErrorCallback = std::function<void(const ApiError &error)>;
    using FinishedCallback = std::function<void()>;

    explicit HermindSseClient(QNetworkAccessManager *manager, QObject *parent = nullptr);

    void start(const QUrl &url,
               const QJsonObject &body,
               const QByteArray &authHeader,
               EventCallback onEvent,
               ErrorCallback onError,
               FinishedCallback onFinished);
    void stop();

private slots:
    void onReadyRead();
    void onReplyFinished();

private:
    void flushEvent();

    QNetworkAccessManager *m_manager = nullptr;
    QNetworkReply *m_reply = nullptr;
    QByteArray m_buffer;
    QByteArray m_eventData;
    EventCallback m_onEvent;
    ErrorCallback m_onError;
    FinishedCallback m_onFinished;
    bool m_finishedEmitted = false;
};

#endif // HERMIND_SSE_CLIENT_H
