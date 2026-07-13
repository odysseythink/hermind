#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include "hermind_sse_client.h"

class TestSseClient : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void receivesMultipleEvents();
    void abortStopsStream();
    void stop_deletesReply();

private:
    class MockSseServer;
    MockSseServer *m_server = nullptr;
    QNetworkAccessManager *m_manager = nullptr;
};

class TestSseClient::MockSseServer : public QTcpServer
{
    Q_OBJECT
public:
    using Handler = std::function<QByteArray(const QString &path, const QHash<QString, QString> &headers)>;
    explicit MockSseServer(QObject *parent = nullptr) : QTcpServer(parent) {}
    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }
    void setHandler(Handler h) { m_handler = std::move(h); }
    QHash<QString, QString> lastHeaders;
    // When true, keep the connection open without responding (simulates a
    // long-lived SSE stream that never ends on its own).
    bool hang = false;

protected:
    void incomingConnection(qintptr socketDescriptor) override
    {
        QTcpSocket *socket = new QTcpSocket(this);
        if (hang) {
            m_heldSockets.append(socket);
            socket->setSocketDescriptor(socketDescriptor);
            return;
        }
        connect(socket, &QTcpSocket::readyRead, this, [this, socket]() {
            QByteArray data = socket->readAll();
            QString text = QString::fromUtf8(data);
            QStringList lines = text.split(QStringLiteral("\r\n"));
            if (lines.size() < 2)
                lines = text.split(QStringLiteral("\n"));
            if (lines.isEmpty()) {
                socket->close();
                socket->deleteLater();
                return;
            }

            QStringList parts = lines.first().split(' ');
            QString encodedPath = parts.value(1);
            QString path = QUrl::fromPercentEncoding(encodedPath.toUtf8());

            QHash<QString, QString> headers;
            for (int i = 1; i < lines.size(); ++i) {
                QString line = lines.at(i);
                if (line.isEmpty())
                    break;
                int colon = line.indexOf(':');
                if (colon > 0) {
                    headers.insert(line.left(colon).trimmed().toLower(),
                                   line.mid(colon + 1).trimmed());
                }
            }
            lastHeaders = headers;

            QByteArray body = m_handler ? m_handler(path, headers)
                                        : QByteArray("data: {}\n\n");
            QByteArray response = "HTTP/1.1 200 OK\r\n"
                                  "Content-Type: text/event-stream\r\n"
                                  "Cache-Control: no-cache\r\n"
                                  "Connection: close\r\n"
                                  "Content-Length: "
                                  + QByteArray::number(body.size())
                                  + "\r\n\r\n"
                                  + body;
            socket->write(response);
            socket->flush();
            socket->close();
            socket->deleteLater();
        });
        socket->setSocketDescriptor(socketDescriptor);
    }

private:
    Handler m_handler;
    QVector<QTcpSocket *> m_heldSockets;
};

void TestSseClient::initTestCase()
{
    m_server = new MockSseServer(this);
    QVERIFY(m_server->start());
    m_manager = new QNetworkAccessManager(this);
}

void TestSseClient::cleanupTestCase()
{
    delete m_manager;
    delete m_server;
}

void TestSseClient::receivesMultipleEvents()
{
    QByteArray streamBody;
    streamBody += "data: {\"uuid\":\"1\",\"type\":\"textResponseChunk\",\"textResponse\":\"hello \",\"close\":false}\n\n";
    streamBody += "data: {\"uuid\":\"1\",\"type\":\"finalizeResponseStream\",\"close\":true}\n\n";

    m_server->setHandler([&](const QString &, const QHash<QString, QString> &) {
        return streamBody;
    });

    QVector<HermindStreamChatResponse> events;
    bool finished = false;

    HermindSseClient client(m_manager);
    client.start(QUrl(QStringLiteral("http://127.0.0.1:%1/api/workspace/default/stream-chat")
                          .arg(m_server->port())),
                 QJsonObject(),
                 QByteArrayLiteral("Bearer jwt"),
                 [&](const HermindStreamChatResponse &r) { events.append(r); },
                 [&](const ApiError &) {},
                 [&]() { finished = true; });

    QTRY_VERIFY_WITH_TIMEOUT(finished, 5000);
    QCOMPARE(events.size(), 2);
    QCOMPARE(events.first().type(), QStringLiteral("textResponseChunk"));
    QVERIFY(events.first().textResponse().has_value());
    QCOMPARE(events.first().textResponse().value(), QStringLiteral("hello "));
    QCOMPARE(events.last().type(), QStringLiteral("finalizeResponseStream"));
    QCOMPARE(events.last().close(), true);
    QCOMPARE(m_server->lastHeaders.value(QStringLiteral("authorization")),
             QStringLiteral("Bearer jwt"));
    QCOMPARE(m_server->lastHeaders.value(QStringLiteral("accept")),
             QStringLiteral("text/event-stream"));
}

void TestSseClient::abortStopsStream()
{
    QByteArray streamBody;
    streamBody += "data: {\"uuid\":\"1\",\"type\":\"textResponseChunk\",\"textResponse\":\"x\",\"close\":false}\n\n";
    streamBody += "data: {\"uuid\":\"1\",\"type\":\"finalizeResponseStream\",\"close\":true}\n\n";

    m_server->setHandler([&](const QString &, const QHash<QString, QString> &) {
        return streamBody;
    });

    QVector<HermindStreamChatResponse> events;
    bool finished = false;

    HermindSseClient client(m_manager);
    client.start(QUrl(QStringLiteral("http://127.0.0.1:%1/api/workspace/default/stream-chat")
                          .arg(m_server->port())),
                 QJsonObject(),
                 QByteArray(),
                 [&](const HermindStreamChatResponse &r) { events.append(r); },
                 [&](const ApiError &) {},
                 [&]() { finished = true; });

    client.stop();

    QTRY_VERIFY_WITH_TIMEOUT(finished, 5000);
    QVERIFY(events.size() < 2);
}

void TestSseClient::stop_deletesReply()
{
    m_server->hang = true;

    HermindSseClient client(m_manager);
    client.start(QUrl(QStringLiteral("http://127.0.0.1:%1/api/workspace/default/stream-chat")
                          .arg(m_server->port())),
                 QJsonObject(),
                 QByteArray(),
                 [](const HermindStreamChatResponse &) {},
                 [](const ApiError &) {},
                 []() {});

    QTRY_VERIFY_WITH_TIMEOUT(!m_manager->findChildren<QNetworkReply *>().isEmpty(), 5000);

    client.stop();

    // stop() must release the QNetworkReply (deleteLater), not just abort it.
    QTRY_VERIFY_WITH_TIMEOUT(m_manager->findChildren<QNetworkReply *>().isEmpty(), 5000);

    m_server->hang = false;
}

QTEST_MAIN(TestSseClient)
#include "tst_sse_client.moc"
