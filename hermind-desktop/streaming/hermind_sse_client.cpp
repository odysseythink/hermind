#include "hermind_sse_client.h"

#include <QNetworkAccessManager>
#include <QNetworkRequest>
#include <QJsonDocument>

HermindSseClient::HermindSseClient(QNetworkAccessManager *manager, QObject *parent)
    : QObject(parent)
    , m_manager(manager)
{
}

void HermindSseClient::start(const QUrl &url,
                             const QJsonObject &body,
                             const QByteArray &authHeader,
                             EventCallback onEvent,
                             ErrorCallback onError,
                             FinishedCallback onFinished)
{
    m_onEvent = std::move(onEvent);
    m_onError = std::move(onError);
    m_onFinished = std::move(onFinished);
    m_buffer.clear();
    m_eventData.clear();
    m_finishedEmitted = false;

    QNetworkRequest request(url);
    request.setHeader(QNetworkRequest::ContentTypeHeader,
                      QStringLiteral("application/json"));
    request.setRawHeader(QByteArrayLiteral("Accept"),
                         QByteArrayLiteral("text/event-stream"));
    if (!authHeader.isEmpty())
        request.setRawHeader(QByteArrayLiteral("Authorization"), authHeader);

    const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
    m_reply = m_manager->post(request, payload);

    connect(m_reply, &QNetworkReply::readyRead, this, &HermindSseClient::onReadyRead);
    connect(m_reply, &QNetworkReply::finished, this, &HermindSseClient::onReplyFinished);
}

void HermindSseClient::stop()
{
    if (m_reply) {
        disconnect(m_reply, nullptr, this, nullptr);
        m_reply->abort();
        m_reply = nullptr;
    }
    if (!m_finishedEmitted && m_onFinished) {
        m_finishedEmitted = true;
        m_onFinished();
    }
}

void HermindSseClient::onReadyRead()
{
    if (!m_reply)
        return;

    m_buffer += m_reply->readAll();

    int pos = 0;
    while ((pos = m_buffer.indexOf('\n')) != -1) {
        QByteArray line = m_buffer.left(pos);
        m_buffer.remove(0, pos + 1);
        if (line.endsWith('\r'))
            line.chop(1);

        if (line.isEmpty()) {
            flushEvent();
        } else if (line.startsWith("data:")) {
            QByteArray data = line.mid(5);
            if (data.startsWith(' '))
                data.remove(0, 1);
            if (!m_eventData.isEmpty())
                m_eventData += '\n';
            m_eventData += data;
        }
        // ignore lines starting with "id:" / "event:" / ":"
    }
}

void HermindSseClient::flushEvent()
{
    if (m_eventData.isEmpty())
        return;

    QJsonParseError parseErr;
    QJsonDocument doc = QJsonDocument::fromJson(m_eventData, &parseErr);
    m_eventData.clear();

    if (parseErr.error == QJsonParseError::NoError && doc.isObject() && m_onEvent) {
        m_onEvent(HermindStreamChatResponse::fromJson(doc.object()));
    }
}

void HermindSseClient::onReplyFinished()
{
    if (!m_reply)
        return;

    flushEvent();

    if (m_reply->error() != QNetworkReply::NoError &&
        m_reply->error() != QNetworkReply::OperationCanceledError && m_onError) {
        const int status = m_reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        m_onError(ApiError(m_reply->errorString(), status, m_reply->error()));
    }

    m_reply->deleteLater();
    m_reply = nullptr;

    if (!m_finishedEmitted && m_onFinished) {
        m_finishedEmitted = true;
        m_onFinished();
    }
}
