#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QJsonDocument>
#include <QWebSocketServer>
#include <QWebSocket>
#include "hermind_api_client.h"
#include "hermind_stream_chat_response.h"

class TestApiClient : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void baseUrlAndAuthToken();
    void genericGet();
    void genericPost();
    void genericDelete();
    void authHeaderInjected();
    void requestToken();
    void requestTokenInvalid();
    void refreshUser();
    void refreshUserSingleUser();
    void listWorkspaces();
    void createWorkspace();
    void getWorkspace();
    void listThreads();
    void createThread();
    void updateThread();
    void deleteThread();

    void streamChat();
    void streamThreadChat();
    void abortStream();
    void openAgentWebSocket();

private:
    class MockHttpServer;
    MockHttpServer *m_server = nullptr;
    HermindApiClient *m_client = nullptr;
    QWebSocketServer *m_wsServer = nullptr;
    QWebSocket *m_wsSocket = nullptr;
};

void TestApiClient::baseUrlAndAuthToken()
{
    HermindApiClient client;
    QCOMPARE(client.baseUrl(), QUrl(QStringLiteral("http://localhost:3001/api")));

    QUrl custom(QStringLiteral("http://127.0.0.1:9999/api"));
    client.setBaseUrl(custom);
    QCOMPARE(client.baseUrl(), custom);

    client.setAuthToken(QStringLiteral("jwt-token"));
    QCOMPARE(client.authToken(), QStringLiteral("jwt-token"));
}

class TestApiClient::MockHttpServer : public QTcpServer
{
    Q_OBJECT
public:
    using Handler = std::function<QByteArray(const QString &method,
                                             const QString &path,
                                             const QHash<QString, QString> &headers,
                                             const QByteArray &body)>;
    explicit MockHttpServer(QObject *parent = nullptr) : QTcpServer(parent) {}
    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }
    void setHandler(Handler h) { m_handler = std::move(h); }

    QVector<QString> recordedPaths;
    QHash<QString, QString> lastHeaders;

protected:
    void incomingConnection(qintptr socketDescriptor) override
    {
        QTcpSocket *socket = new QTcpSocket(this);
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
            QString method = parts.value(0);
            QString encodedPath = parts.value(1);
            QString path = QUrl::fromPercentEncoding(encodedPath.toUtf8());
            recordedPaths.append(path);

            QHash<QString, QString> headers;
            int i = 1;
            for (; i < lines.size(); ++i) {
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

            QByteArray body;
            int blank = data.indexOf("\r\n\r\n");
            int step = 4;
            if (blank < 0) {
                blank = data.indexOf("\n\n");
                step = 2;
            }
            if (blank >= 0)
                body = data.mid(blank + step);

            QByteArray respBody = m_handler ? m_handler(method, path, headers, body)
                                            : QByteArray("{}");
            QByteArray response = "HTTP/1.1 200 OK\r\n"
                                  "Content-Type: application/json\r\n"
                                  "Content-Length: "
                                  + QByteArray::number(respBody.size())
                                  + "\r\nConnection: close\r\n\r\n"
                                  + respBody;
            socket->write(response);
            socket->flush();
            socket->close();
            socket->deleteLater();
        });
        socket->setSocketDescriptor(socketDescriptor);
    }

private:
    Handler m_handler;
};

void TestApiClient::initTestCase()
{
    m_server = new MockHttpServer(this);
    QVERIFY(m_server->start());

    m_client = new HermindApiClient(this);
    QUrl url(QStringLiteral("http://127.0.0.1:%1/api").arg(m_server->port()));
    m_client->setBaseUrl(url);

    m_wsServer = new QWebSocketServer(QStringLiteral("api-test"),
                                       QWebSocketServer::NonSecureMode,
                                       this);
    QVERIFY(m_wsServer->listen(QHostAddress::LocalHost, 0));
    connect(m_wsServer, &QWebSocketServer::newConnection, this, [&]() {
        m_wsSocket = m_wsServer->nextPendingConnection();
        connect(m_wsSocket, &QWebSocket::textMessageReceived,
                this, [&](const QString &message) {
                    if (m_wsSocket)
                        m_wsSocket->sendTextMessage(message);
                });
    });
}

void TestApiClient::cleanupTestCase()
{
    if (m_wsSocket) {
        m_wsSocket->close();
        delete m_wsSocket;
        m_wsSocket = nullptr;
    }
    if (m_wsServer) {
        m_wsServer->close();
        delete m_wsServer;
        m_wsServer = nullptr;
    }
    delete m_client;
    delete m_server;
}

void TestApiClient::genericGet()
{
    bool done = false;
    ApiResponse response;
    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &, const QByteArray &) {
        return QByteArray(R"({"ok":true})");
    });

    m_client->get(QStringLiteral("/workspaces"), QUrlQuery(),
                  [&](const ApiResponse &r) { done = true; response = r; });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(response.isSuccess());
    QCOMPARE(response.statusCode(), 200);
    QVERIFY(response.body().object().value(QStringLiteral("ok")).toBool());
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspaces"));
}

void TestApiClient::genericPost()
{
    bool done = false;
    ApiResponse response;
    QByteArray receivedBody;
    m_server->recordedPaths.clear();
    m_server->setHandler([&](const QString &, const QString &, const QHash<QString, QString> &,
                             const QByteArray &body) {
        receivedBody = body;
        return QByteArray(R"({"id":1})");
    });

    QJsonObject payload;
    payload.insert(QStringLiteral("name"), QStringLiteral("test"));
    m_client->post(QStringLiteral("/workspace/new"), payload,
                   [&](const ApiResponse &r) { done = true; response = r; });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(response.isSuccess());
    QCOMPARE(response.statusCode(), 200);
    QCOMPARE(response.body().object().value(QStringLiteral("id")).toInt(), 1);
    QCOMPARE(receivedBody, QJsonDocument(payload).toJson(QJsonDocument::Compact));
}

void TestApiClient::genericDelete()
{
    bool done = false;
    ApiResponse response;
    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &method, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return method == QStringLiteral("DELETE")
               ? QByteArray("{}")
               : QByteArray(R"({"error":"expected DELETE"})");
    });

    m_client->del(QStringLiteral("/workspace/old"), QJsonObject(),
                  [&](const ApiResponse &r) { done = true; response = r; });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(response.isSuccess());
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/old"));
}

void TestApiClient::authHeaderInjected()
{
    bool done = false;
    m_server->recordedPaths.clear();
    m_server->lastHeaders.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray("{}");
    });

    m_client->setAuthToken(QStringLiteral("test-jwt"));
    m_client->get(QStringLiteral("/system/check-token"), QUrlQuery(),
                  [&](const ApiResponse &) { done = true; });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QCOMPARE(m_server->lastHeaders.value(QStringLiteral("authorization")),
             QStringLiteral("Bearer test-jwt"));
}

void TestApiClient::requestToken()
{
    bool done = false;
    QString token;
    QString message;
    ApiError err;
    QByteArray receivedBody;

    m_server->recordedPaths.clear();
    m_server->setHandler([&](const QString &, const QString &, const QHash<QString, QString> &,
                             const QByteArray &body) {
        receivedBody = body;
        return QByteArray(R"({"valid":true,"token":"jwt-123","message":null})");
    });

    m_client->requestToken(QStringLiteral("alice"), QStringLiteral("secret"),
                           [&](const QString &t, const QString &m, const ApiError &e) {
                               done = true;
                               token = t;
                               message = m;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(token, QStringLiteral("jwt-123"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/request-token"));

    QJsonObject expectedBody;
    expectedBody.insert(QStringLiteral("username"), QStringLiteral("alice"));
    expectedBody.insert(QStringLiteral("password"), QStringLiteral("secret"));
    QCOMPARE(receivedBody, QJsonDocument(expectedBody).toJson(QJsonDocument::Compact));
}

void TestApiClient::requestTokenInvalid()
{
    bool done = false;
    QString token;
    QString message;
    ApiError err;

    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"valid":false,"token":"","message":"bad credentials"})");
    });

    m_client->requestToken(QStringLiteral("alice"), QStringLiteral("wrong"),
                           [&](const QString &t, const QString &m, const ApiError &e) {
                               done = true;
                               token = t;
                               message = m;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(!err.isEmpty());
    QVERIFY(token.isEmpty());
    QCOMPARE(err.message(), QStringLiteral("bad credentials"));
}

void TestApiClient::refreshUser()
{
    bool done = false;
    HermindUser user;
    QString message;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"success":true,"user":{"id":7,"username":"alice","role":"admin","suspended":0}})");
    });

    m_client->setAuthToken(QStringLiteral("jwt-123"));
    m_client->refreshUser([&](const HermindUser &u, const QString &m, const ApiError &e) {
        done = true;
        user = u;
        message = m;
        err = e;
    });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(user.id(), 7);
    QCOMPARE(user.username(), QStringLiteral("alice"));
    QCOMPARE(user.role(), QStringLiteral("admin"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/system/refresh-user"));
    QCOMPARE(m_server->lastHeaders.value(QStringLiteral("authorization")),
             QStringLiteral("Bearer jwt-123"));
}

void TestApiClient::refreshUserSingleUser()
{
    bool done = false;
    HermindUser user;
    ApiError err;

    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"success":true,"user":null,"message":null})");
    });

    m_client->refreshUser([&](const HermindUser &u, const QString &, const ApiError &e) {
        done = true;
        user = u;
        err = e;
    });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(user.id(), 0);
}

void TestApiClient::listWorkspaces()
{
    bool done = false;
    QVector<HermindWorkspace> workspaces;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"workspaces":[{"id":1,"name":"Default","slug":"default","openAiHistory":20}]})");
    });

    m_client->listWorkspaces([&](const QVector<HermindWorkspace> &list, const ApiError &e) {
        done = true;
        workspaces = list;
        err = e;
    });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(workspaces.size(), 1);
    QCOMPARE(workspaces.first().name(), QStringLiteral("Default"));
    QCOMPARE(workspaces.first().slug(), QStringLiteral("default"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspaces"));
}

void TestApiClient::createWorkspace()
{
    bool done = false;
    HermindWorkspace workspace;
    QString message;
    ApiError err;
    QByteArray receivedBody;

    m_server->recordedPaths.clear();
    m_server->setHandler([&](const QString &, const QString &, const QHash<QString, QString> &,
                             const QByteArray &body) {
        receivedBody = body;
        return QByteArray(R"({"workspace":{"id":2,"name":"New Workspace","slug":"new-workspace"},"message":"Workspace created"})");
    });

    m_client->createWorkspace(QStringLiteral("New Workspace"),
                              [&](const HermindWorkspace &ws, const QString &m, const ApiError &e) {
                                  done = true;
                                  workspace = ws;
                                  message = m;
                                  err = e;
                              });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(workspace.name(), QStringLiteral("New Workspace"));
    QCOMPARE(workspace.slug(), QStringLiteral("new-workspace"));
    QCOMPARE(message, QStringLiteral("Workspace created"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/new"));

    QJsonObject expectedBody;
    expectedBody.insert(QStringLiteral("name"), QStringLiteral("New Workspace"));
    QCOMPARE(receivedBody, QJsonDocument(expectedBody).toJson(QJsonDocument::Compact));
}

void TestApiClient::getWorkspace()
{
    bool done = false;
    HermindWorkspace workspace;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"workspace":{"id":1,"name":"Default","slug":"default"}})");
    });

    m_client->getWorkspace(QStringLiteral("default"),
                           [&](const HermindWorkspace &ws, const QString &, const ApiError &e) {
                               done = true;
                               workspace = ws;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(workspace.id(), 1);
    QCOMPARE(workspace.slug(), QStringLiteral("default"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/default"));
}

void TestApiClient::listThreads()
{
    bool done = false;
    QVector<HermindWorkspaceThread> threads;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"threads":[{"id":1,"name":"Thread A","slug":"thread-a","workspaceId":1}]})");
    });

    m_client->listThreads(QStringLiteral("default"),
                          [&](const QVector<HermindWorkspaceThread> &list, const ApiError &e) {
                              done = true;
                              threads = list;
                              err = e;
                          });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(threads.size(), 1);
    QCOMPARE(threads.first().name(), QStringLiteral("Thread A"));
    QCOMPARE(threads.first().slug(), QStringLiteral("thread-a"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/default/threads"));
}

void TestApiClient::createThread()
{
    bool done = false;
    HermindWorkspaceThread thread;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return QByteArray(R"({"thread":{"id":2,"name":"New Thread","slug":"new-thread","workspaceId":1}})");
    });

    m_client->createThread(QStringLiteral("default"),
                           [&](const HermindWorkspaceThread &t, const QString &, const ApiError &e) {
                               done = true;
                               thread = t;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(thread.slug(), QStringLiteral("new-thread"));
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/default/thread/new"));
}

void TestApiClient::updateThread()
{
    bool done = false;
    HermindWorkspaceThread thread;
    ApiError err;
    QByteArray receivedBody;

    m_server->recordedPaths.clear();
    m_server->setHandler([&](const QString &, const QString &, const QHash<QString, QString> &,
                             const QByteArray &body) {
        receivedBody = body;
        return QByteArray(R"({"thread":{"id":1,"name":"Renamed","slug":"thread-a","workspaceId":1}})");
    });

    m_client->updateThread(QStringLiteral("default"), QStringLiteral("thread-a"),
                           QStringLiteral("Renamed"),
                           [&](const HermindWorkspaceThread &t, const QString &, const ApiError &e) {
                               done = true;
                               thread = t;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(err.isEmpty());
    QCOMPARE(thread.name(), QStringLiteral("Renamed"));
    QCOMPARE(m_server->recordedPaths.last(),
             QStringLiteral("/api/workspace/default/thread/thread-a/update"));

    QJsonObject expectedBody;
    expectedBody.insert(QStringLiteral("name"), QStringLiteral("Renamed"));
    QCOMPARE(receivedBody, QJsonDocument(expectedBody).toJson(QJsonDocument::Compact));
}

void TestApiClient::deleteThread()
{
    bool done = false;
    bool success = false;
    ApiError err;

    m_server->recordedPaths.clear();
    m_server->setHandler([](const QString &method, const QString &, const QHash<QString, QString> &,
                            const QByteArray &) {
        return method == QStringLiteral("DELETE") ? QByteArray("{}")
                                                  : QByteArray(R"({"error":"expected DELETE"})");
    });

    m_client->deleteThread(QStringLiteral("default"), QStringLiteral("thread-a"),
                           [&](bool ok, const ApiError &e) {
                               done = true;
                               success = ok;
                               err = e;
                           });

    QTRY_VERIFY_WITH_TIMEOUT(done, 5000);
    QVERIFY(success);
    QVERIFY(err.isEmpty());
    QCOMPARE(m_server->recordedPaths.last(), QStringLiteral("/api/workspace/default/thread/thread-a"));
}

void TestApiClient::streamChat()
{
    QByteArray capturedMethod;
    QString capturedPath;
    QByteArray capturedBody;
    QVector<HermindStreamChatResponse> chunks;
    bool finished = false;

    m_server->recordedPaths.clear();
    m_server->setHandler([&](const QString &method, const QString &path,
                             const QHash<QString, QString> &, const QByteArray &body) {
        capturedMethod = method.toUtf8();
        capturedPath = path;
        capturedBody = body;
        return QByteArray("data: {\"uuid\":\"1\",\"type\":\"textResponseChunk\",\"textResponse\":\"hi\",\"close\":false}\n\n"
                          "data: {\"uuid\":\"1\",\"type\":\"finalizeResponseStream\",\"close\":true}\n\n");
    });

    m_client->setAuthToken(QStringLiteral("jwt-123"));
    m_client->streamChat(QStringLiteral("default"),
                         QStringLiteral("hello"),
                         QStringList(),
                         [&](const HermindStreamChatResponse &r) { chunks.append(r); },
                         [&](const ApiError &) {},
                         [&]() { finished = true; });

    QTRY_VERIFY_WITH_TIMEOUT(finished, 5000);
    QCOMPARE(capturedMethod, QByteArrayLiteral("POST"));
    QCOMPARE(capturedPath, QStringLiteral("/api/workspace/default/stream-chat"));
    QCOMPARE(chunks.size(), 2);
    QCOMPARE(chunks.first().type(), QStringLiteral("textResponseChunk"));
    QCOMPARE(chunks.last().type(), QStringLiteral("finalizeResponseStream"));

    QJsonObject expectedBody;
    expectedBody.insert(QStringLiteral("message"), QStringLiteral("hello"));
    expectedBody.insert(QStringLiteral("attachments"), QJsonArray());
    QCOMPARE(capturedBody, QJsonDocument(expectedBody).toJson(QJsonDocument::Compact));
}

void TestApiClient::streamThreadChat()
{
    QString capturedPath;
    bool finished = false;

    m_server->setHandler([&](const QString &, const QString &path,
                             const QHash<QString, QString> &, const QByteArray &) {
        capturedPath = path;
        return QByteArray("data: {\"uuid\":\"2\",\"type\":\"textResponse\",\"textResponse\":\"ok\",\"close\":true}\n\n");
    });

    m_client->streamThreadChat(QStringLiteral("default"),
                               QStringLiteral("thread-a"),
                               QStringLiteral("q"),
                               QStringList(),
                               [&](const HermindStreamChatResponse &) {},
                               [&](const ApiError &) {},
                               [&]() { finished = true; });

    QTRY_VERIFY_WITH_TIMEOUT(finished, 5000);
    QCOMPARE(capturedPath, QStringLiteral("/api/workspace/default/thread/thread-a/stream-chat"));
}

void TestApiClient::abortStream()
{
    bool finished = false;
    QVector<HermindStreamChatResponse> chunks;

    m_server->setHandler([&](const QString &, const QString &, const QHash<QString, QString> &,
                             const QByteArray &) {
        return QByteArray("data: {\"uuid\":\"3\",\"type\":\"textResponseChunk\",\"textResponse\":\"x\",\"close\":false}\n\n"
                          "data: {\"uuid\":\"3\",\"type\":\"finalizeResponseStream\",\"close\":true}\n\n");
    });

    m_client->streamChat(QStringLiteral("default"),
                         QStringLiteral("hello"),
                         QStringList(),
                         [&](const HermindStreamChatResponse &r) { chunks.append(r); },
                         [&](const ApiError &) {},
                         [&]() { finished = true; });

    m_client->abortStream();
    QTRY_VERIFY_WITH_TIMEOUT(finished, 5000);
    QVERIFY(chunks.size() < 2);
}

void TestApiClient::openAgentWebSocket()
{
    bool opened = false;
    bool closed = false;
    QJsonObject received;

    // Point baseUrl to the WebSocket test server's port
    m_client->setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api")
                                .arg(m_wsServer->serverPort())));

    m_client->openAgentWebSocket(QStringLiteral("uuid-1"),
                                 QStringLiteral("tok-1"),
                                 [&](const QJsonObject &obj) { received = obj; },
                                 [&](const QString &) {},
                                 [&]() { closed = true; },
                                 [&]() { opened = true; });

    QTRY_VERIFY_WITH_TIMEOUT(opened, 5000);

    m_client->sendAgentFeedback(QStringLiteral("go ahead"), QStringList());
    QTRY_VERIFY_WITH_TIMEOUT(received.value(QStringLiteral("feedback")).toString() ==
                                 QStringLiteral("go ahead"),
                             5000);

    m_client->closeAgentWebSocket();
    QTRY_VERIFY_WITH_TIMEOUT(closed, 5000);
}

QTEST_MAIN(TestApiClient)
#include "tst_api_client.moc"
