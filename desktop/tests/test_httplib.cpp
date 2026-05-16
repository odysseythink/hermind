#include <QTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QHostAddress>
#include <QCoreApplication>
#include <QJSEngine>
#include "../src/HermindClient.h"

class TestHttpLib : public QObject
{
    Q_OBJECT
private slots:
    void testBaseUrl();
    void testPut();
    void testDelete();
    void testUpload();
};

void TestHttpLib::testBaseUrl()
{
    HermindClient client("http://127.0.0.1:12345");
    QCOMPARE(client.baseUrl(), QString("http://127.0.0.1:12345"));
}

void TestHttpLib::testPut()
{
    HermindClient client("http://127.0.0.1:12345");
    QNetworkAccessManager *manager = client.findChild<QNetworkAccessManager*>();
    QVERIFY(manager);
    QSignalSpy spy(manager, &QNetworkAccessManager::finished);

    QJSEngine engine;
    client.put("/test", QJsonObject{{"key", "value"}}, engine.evaluate("(function(){ })"));

    QVERIFY(spy.wait(5000));
    QNetworkReply *reply = qvariant_cast<QNetworkReply*>(spy.at(0).at(0));
    QVERIFY(reply);
    QCOMPARE(reply->operation(), QNetworkAccessManager::PutOperation);
    QCOMPARE(reply->request().url().path(), QString("/test"));
}

void TestHttpLib::testDelete()
{
    HermindClient client("http://127.0.0.1:12345");
    QNetworkAccessManager *manager = client.findChild<QNetworkAccessManager*>();
    QVERIFY(manager);
    QSignalSpy spy(manager, &QNetworkAccessManager::finished);

    QJSEngine engine;
    client.delete_("/test", engine.evaluate("(function(){ })"));

    QVERIFY(spy.wait(5000));
    QNetworkReply *reply = qvariant_cast<QNetworkReply*>(spy.at(0).at(0));
    QVERIFY(reply);
    QCOMPARE(reply->operation(), QNetworkAccessManager::DeleteOperation);
    QCOMPARE(reply->request().url().path(), QString("/test"));
}

void TestHttpLib::testUpload()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    int port = server.serverPort();

    HermindClient client(QString("http://127.0.0.1:%1").arg(port));
    QNetworkAccessManager *manager = client.findChild<QNetworkAccessManager*>();
    QVERIFY(manager);
    QSignalSpy finishedSpy(manager, &QNetworkAccessManager::finished);
    QSignalSpy connSpy(&server, &QTcpServer::newConnection);

    QJSEngine engine;
    client.upload("/upload", QByteArray("hello"), "test.txt", "text/plain",
                  engine.evaluate("(function(){ })"));

    QVERIFY(connSpy.wait(5000));
    QTcpSocket *socket = server.nextPendingConnection();
    QVERIFY(socket);

    QByteArray request;
    for (int i = 0; i < 30 && request.indexOf("\r\n\r\n") == -1; ++i) {
        QTest::qWait(100);
        request.append(socket->readAll());
    }
    for (int i = 0; i < 10; ++i) {
        QTest::qWait(100);
        request.append(socket->readAll());
    }

    QVERIFY(request.contains("Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\""));
    QVERIFY(request.contains("Content-Type: text/plain"));
    QVERIFY(request.contains("hello"));
    QVERIFY(request.contains("------HermindBoundary"));

    socket->write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Type: application/json\r\n\r\n{}");
    socket->flush();

    QVERIFY(finishedSpy.wait(5000));
    QNetworkReply *reply = qvariant_cast<QNetworkReply*>(finishedSpy.at(0).at(0));
    QVERIFY(reply);
    QCOMPARE(reply->operation(), QNetworkAccessManager::PostOperation);

    socket->close();
    delete socket;
}

QTEST_MAIN(TestHttpLib)
#include "test_httplib.moc"
