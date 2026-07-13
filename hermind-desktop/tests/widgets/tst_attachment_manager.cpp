#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QFile>
#include <QImage>
#include "attachment_manager.h"
#include "hermind_api_client.h"

class MockUploadServer : public QTcpServer
{
    Q_OBJECT
public:
    std::function<QByteArray(const QString &method, const QString &path)> handler;
    QVector<QString> requests;

    bool start() { return listen(QHostAddress::LocalHost, 0); }

protected:
    void incomingConnection(qintptr fd) override
    {
        QTcpSocket *socket = new QTcpSocket(this);
        connect(socket, &QTcpSocket::readyRead, this, [this, socket]() {
            const QByteArray data = socket->readAll();
            const QString firstLine = QString::fromUtf8(data.left(data.indexOf("\r\n")));
            const QStringList parts = firstLine.split(QLatin1Char(' '));
            const QString method = parts.value(0);
            const QString path = QUrl::fromPercentEncoding(parts.value(1).toUtf8());
            requests.append(method + QLatin1Char(' ') + path);
            const QByteArray respBody = handler ? handler(method, path) : QByteArray("{}");
            const QByteArray response = QByteArrayLiteral("HTTP/1.1 200 OK\r\n"
                                                          "Content-Type: application/json\r\n"
                                                          "Content-Length: ")
                                        + QByteArray::number(respBody.size())
                                        + QByteArrayLiteral("\r\nConnection: close\r\n\r\n") + respBody;
            socket->write(response);
            socket->flush();
            socket->close();
            socket->deleteLater();
        });
        socket->setSocketDescriptor(fd);
    }
};

class TestAttachmentManager : public QObject
{
    Q_OBJECT
private slots:
    void startsEmpty();
    void addFilesEmitsSignal();
    void removeFileEmitsSignal();
    void clearResets();
    void imageFile_convertedToDataUrl();
    void nonImageFile_withoutClient_failsImmediately();
    void nonImageFile_uploadsAndEmbeds();
    void embeddedDoc_removeFile_callsRemoveAndUnembed();
    void failedUpload_stopsProcessing();

private:
    static QString writeTextFile(QTemporaryDir &dir, const QString &name);
};

QString TestAttachmentManager::writeTextFile(QTemporaryDir &dir, const QString &name)
{
    const QString path = dir.path() + QLatin1Char('/') + name;
    QFile f(path);
    f.open(QIODevice::WriteOnly);
    f.write("hi");
    f.close();
    return path;
}

void TestAttachmentManager::startsEmpty()
{
    AttachmentManager mgr;
    QCOMPARE(mgr.count(), 0);
    QVERIFY(mgr.filePaths().isEmpty());
    QVERIFY(!mgr.isProcessing());
    QVERIFY(mgr.imageDataUrls().isEmpty());
}

void TestAttachmentManager::addFilesEmitsSignal()
{
    AttachmentManager mgr;
    QSignalSpy spy(&mgr, &AttachmentManager::attachmentsChanged);

    mgr.addFiles({ QStringLiteral("/tmp/a.txt"), QStringLiteral("/tmp/b.jpg") });

    QCOMPARE(spy.count(), 1);
    QCOMPARE(mgr.count(), 2);
    QStringList paths = spy.at(0).at(0).toStringList();
    QVERIFY(paths.contains(QStringLiteral("/tmp/a.txt")));
    QVERIFY(paths.contains(QStringLiteral("/tmp/b.jpg")));
}

void TestAttachmentManager::removeFileEmitsSignal()
{
    AttachmentManager mgr;
    mgr.addFiles({ QStringLiteral("/tmp/a.txt"), QStringLiteral("/tmp/b.jpg") });

    QSignalSpy spy(&mgr, &AttachmentManager::attachmentsChanged);
    mgr.removeFile(QStringLiteral("/tmp/a.txt"));

    QCOMPARE(spy.count(), 1);
    QCOMPARE(mgr.count(), 1);
    QVERIFY(!mgr.filePaths().contains(QStringLiteral("/tmp/a.txt")));
}

void TestAttachmentManager::clearResets()
{
    AttachmentManager mgr;
    mgr.addFiles({ QStringLiteral("/tmp/a.txt") });
    mgr.clear();
    QCOMPARE(mgr.count(), 0);
}

void TestAttachmentManager::imageFile_convertedToDataUrl()
{
    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = dir.path() + QStringLiteral("/pic.png");
    QImage img(4, 4, QImage::Format_ARGB32);
    img.fill(Qt::red);
    QVERIFY(img.save(path, "PNG"));

    AttachmentManager mgr;
    mgr.addFiles({ path });

    QCOMPARE(mgr.count(), 1);
    QVERIFY(!mgr.isProcessing());
    QCOMPARE(mgr.imageDataUrls().size(), 1);
    QVERIFY(mgr.imageDataUrls().first().startsWith(QStringLiteral("data:image/png;base64,")));
}

void TestAttachmentManager::nonImageFile_withoutClient_failsImmediately()
{
    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = writeTextFile(dir, QStringLiteral("note.txt"));

    AttachmentManager mgr;
    mgr.addFiles({ path });

    QVERIFY(!mgr.isProcessing());
    QVERIFY(mgr.imageDataUrls().isEmpty());
    QCOMPARE(mgr.count(), 1); // kept so the user can see the failed chip
}

void TestAttachmentManager::nonImageFile_uploadsAndEmbeds()
{
    MockUploadServer server;
    QVERIFY(server.start());
    server.handler = [](const QString &, const QString &) {
        return QByteArray(R"({"success":true,"document":{"docId":"doc-7"}})");
    };

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.serverPort())));

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = writeTextFile(dir, QStringLiteral("note.txt"));

    AttachmentManager mgr;
    mgr.setApiClient(&client);
    mgr.setWorkspaceSlug(QStringLiteral("demo"));
    QSignalSpy procSpy(&mgr, &AttachmentManager::processingChanged);

    mgr.addFiles({ path });
    QVERIFY(mgr.isProcessing());
    QTRY_VERIFY_WITH_TIMEOUT(!mgr.isProcessing(), 5000);

    QVERIFY(!server.requests.isEmpty());
    QVERIFY(server.requests.first().contains(QStringLiteral("/workspace/demo/upload-and-embed")));
    QCOMPARE(procSpy.count(), 2);
    QCOMPARE(procSpy.first().first().toBool(), true);
    QCOMPARE(procSpy.last().first().toBool(), false);
}

void TestAttachmentManager::embeddedDoc_removeFile_callsRemoveAndUnembed()
{
    MockUploadServer server;
    QVERIFY(server.start());
    server.handler = [](const QString &method, const QString &) {
        if (method == QLatin1String("DELETE"))
            return QByteArray(R"({"success":true})");
        return QByteArray(R"({"success":true,"document":{"docId":"doc-9"}})");
    };

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.serverPort())));

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = writeTextFile(dir, QStringLiteral("note.txt"));

    AttachmentManager mgr;
    mgr.setApiClient(&client);
    mgr.setWorkspaceSlug(QStringLiteral("demo"));

    mgr.addFiles({ path });
    QTRY_VERIFY_WITH_TIMEOUT(!mgr.isProcessing(), 5000);
    QCOMPARE(server.requests.size(), 1);

    mgr.removeFile(path);
    QTRY_VERIFY_WITH_TIMEOUT(server.requests.size() >= 2, 5000);
    QVERIFY(server.requests.last().startsWith(QStringLiteral("DELETE ")));
    QVERIFY(server.requests.last().contains(QStringLiteral("docId=doc-9")));
    QCOMPARE(mgr.count(), 0);
}

void TestAttachmentManager::failedUpload_stopsProcessing()
{
    MockUploadServer server;
    QVERIFY(server.start());
    server.handler = [](const QString &, const QString &) {
        return QByteArray(R"({"error":"boom"})"); // no document -> failure
    };

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.serverPort())));

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = writeTextFile(dir, QStringLiteral("note.txt"));

    AttachmentManager mgr;
    mgr.setApiClient(&client);
    mgr.setWorkspaceSlug(QStringLiteral("demo"));

    mgr.addFiles({ path });
    QVERIFY(mgr.isProcessing());
    QTRY_VERIFY_WITH_TIMEOUT(!mgr.isProcessing(), 5000);
    QCOMPARE(mgr.count(), 1); // failed chip remains visible
}

QTEST_MAIN(TestAttachmentManager)
#include "tst_attachment_manager.moc"
