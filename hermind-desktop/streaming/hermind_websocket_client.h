#ifndef HERMIND_WEBSOCKET_CLIENT_H
#define HERMIND_WEBSOCKET_CLIENT_H

#include <QObject>
#include <QUrl>
#include <QJsonObject>
#include <QAbstractSocket>
#include <functional>

class QWebSocket;

class HermindWebSocketClient : public QObject
{
    Q_OBJECT

public:
    using MessageCallback = std::function<void(const QJsonObject &message)>;
    using ErrorCallback = std::function<void(const QString &errorString)>;
    using ClosedCallback = std::function<void()>;
    using OpenedCallback = std::function<void()>;

    explicit HermindWebSocketClient(QObject *parent = nullptr);
    ~HermindWebSocketClient();

    void open(const QUrl &url,
              MessageCallback onMessage,
              ErrorCallback onError,
              ClosedCallback onClosed,
              OpenedCallback onOpened = nullptr);
    void close();
    void sendJson(const QJsonObject &obj);
    bool isOpen() const;

private slots:
    void onConnected();
    void onTextMessageReceived(const QString &message);
    void onError(QAbstractSocket::SocketError error);
    void onDisconnected();

private:
    QWebSocket *m_socket = nullptr;
    MessageCallback m_onMessage;
    ErrorCallback m_onError;
    ClosedCallback m_onClosed;
    OpenedCallback m_onOpened;
};

#endif // HERMIND_WEBSOCKET_CLIENT_H
