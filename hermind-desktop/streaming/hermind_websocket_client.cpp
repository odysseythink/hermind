#include "hermind_websocket_client.h"

#include <QWebSocket>
#include <QJsonDocument>

HermindWebSocketClient::HermindWebSocketClient(QObject *parent)
    : QObject(parent)
    , m_socket(new QWebSocket(QString(), QWebSocketProtocol::VersionLatest, this))
{
}

HermindWebSocketClient::~HermindWebSocketClient() = default;

void HermindWebSocketClient::open(const QUrl &url,
                                  MessageCallback onMessage,
                                  ErrorCallback onError,
                                  ClosedCallback onClosed,
                                  OpenedCallback onOpened)
{
    m_onMessage = std::move(onMessage);
    m_onError = std::move(onError);
    m_onClosed = std::move(onClosed);
    m_onOpened = std::move(onOpened);

    connect(m_socket, &QWebSocket::connected,
            this, &HermindWebSocketClient::onConnected);
    connect(m_socket, &QWebSocket::textMessageReceived,
            this, &HermindWebSocketClient::onTextMessageReceived);
    connect(m_socket, &QWebSocket::errorOccurred,
            this, &HermindWebSocketClient::onError);
    connect(m_socket, &QWebSocket::disconnected,
            this, &HermindWebSocketClient::onDisconnected);

    m_socket->open(url);
}

void HermindWebSocketClient::close()
{
    if (m_socket)
        m_socket->close();
}

void HermindWebSocketClient::sendJson(const QJsonObject &obj)
{
    if (m_socket && m_socket->state() == QAbstractSocket::ConnectedState) {
        m_socket->sendTextMessage(QString::fromUtf8(
            QJsonDocument(obj).toJson(QJsonDocument::Compact)));
    }
}

bool HermindWebSocketClient::isOpen() const
{
    return m_socket && m_socket->state() == QAbstractSocket::ConnectedState;
}

void HermindWebSocketClient::onConnected()
{
    if (m_onOpened)
        m_onOpened();
}

void HermindWebSocketClient::onTextMessageReceived(const QString &message)
{
    if (!m_onMessage)
        return;

    QJsonParseError parseErr;
    QJsonDocument doc = QJsonDocument::fromJson(message.toUtf8(), &parseErr);
    if (parseErr.error == QJsonParseError::NoError && doc.isObject()) {
        m_onMessage(doc.object());
    }
}

void HermindWebSocketClient::onError(QAbstractSocket::SocketError)
{
    if (m_onError)
        m_onError(m_socket->errorString());
}

void HermindWebSocketClient::onDisconnected()
{
    if (m_onClosed)
        m_onClosed();
}
