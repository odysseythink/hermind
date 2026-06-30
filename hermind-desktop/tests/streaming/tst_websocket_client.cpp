#include <QtTest>
#include <QWebSocketServer>
#include <QWebSocket>
#include "hermind_websocket_client.h"

class TestWebSocketClient : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void echoRoundTrip();
    void closeEmitsClosedCallback();

private:
    QWebSocketServer *m_server = nullptr;
    QWebSocket *m_serverSocket = nullptr;
};

void TestWebSocketClient::initTestCase()
{
    m_server = new QWebSocketServer(QStringLiteral("test"),
                                    QWebSocketServer::NonSecureMode,
                                    this);
    QVERIFY(m_server->listen(QHostAddress::LocalHost, 0));

    connect(m_server, &QWebSocketServer::newConnection, this, [&]() {
        m_serverSocket = m_server->nextPendingConnection();
        connect(m_serverSocket, &QWebSocket::textMessageReceived,
                this, [&](const QString &message) {
                    m_serverSocket->sendTextMessage(message);
                });
    });
}

void TestWebSocketClient::cleanupTestCase()
{
    if (m_serverSocket) {
        m_serverSocket->close();
        delete m_serverSocket;
    }
    m_server->close();
    delete m_server;
}

void TestWebSocketClient::echoRoundTrip()
{
    bool opened = false;
    QJsonObject received;
    HermindWebSocketClient client;
    client.open(QUrl(QStringLiteral("ws://127.0.0.1:%1/agent-invocation/uuid-1?token=tok-1")
                         .arg(m_server->serverPort())),
                [&](const QJsonObject &obj) { received = obj; },
                [&](const QString &) {},
                [&]() {},
                [&]() { opened = true; });

    QTRY_VERIFY_WITH_TIMEOUT(opened, 5000);

    QJsonObject sent;
    sent.insert(QStringLiteral("type"), QStringLiteral("awaitingFeedback"));
    sent.insert(QStringLiteral("feedback"), QStringLiteral("go ahead"));
    client.sendJson(sent);

    QTRY_VERIFY_WITH_TIMEOUT(received.value(QStringLiteral("type")).toString() ==
                                 QStringLiteral("awaitingFeedback"),
                             5000);
    QCOMPARE(received.value(QStringLiteral("feedback")).toString(),
             QStringLiteral("go ahead"));
}

void TestWebSocketClient::closeEmitsClosedCallback()
{
    bool opened = false;
    bool closed = false;
    HermindWebSocketClient client;
    client.open(QUrl(QStringLiteral("ws://127.0.0.1:%1/agent-invocation/uuid-2?token=tok-2")
                         .arg(m_server->serverPort())),
                [&](const QJsonObject &) {},
                [&](const QString &) {},
                [&]() { closed = true; },
                [&]() { opened = true; });

    QTRY_VERIFY_WITH_TIMEOUT(opened, 5000);
    client.close();
    QTRY_VERIFY_WITH_TIMEOUT(closed, 5000);
}

QTEST_MAIN(TestWebSocketClient)
#include "tst_websocket_client.moc"
