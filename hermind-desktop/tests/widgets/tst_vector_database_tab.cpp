#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QComboBox>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPushButton>
#include <QSpinBox>

#include "vector_database_tab.h"
#include "hermind_api_client.h"

// Dispatching in-process HTTP server for workspace vector database settings.
class MockVectorDbServer : public QTcpServer
{
    Q_OBJECT
public:
    QByteArray vectorDb = "lancedb";
    QByteArray vectorCountBody = R"({"vectorCount":42})";
    QByteArray lastUpdateBody;
    int updateRequests = 0;

    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }

protected:
    void incomingConnection(qintptr fd) override
    {
        auto *socket = new QTcpSocket(this);
        auto *buffer = new QByteArray();
        connect(socket, &QTcpSocket::readyRead, this, [this, socket, buffer]() {
            buffer->append(socket->readAll());
            const int headerEnd = buffer->indexOf("\r\n\r\n");
            if (headerEnd < 0)
                return;

            const QByteArray header = buffer->left(headerEnd);
            const QList<QByteArray> lines = header.split('\n');
            const QList<QByteArray> requestLine = lines.first().trimmed().split(' ');
            if (requestLine.size() < 2)
                return;
            const QString method = QString::fromUtf8(requestLine.at(0));
            const QString path = QString::fromUtf8(requestLine.at(1));

            int contentLength = 0;
            for (const QByteArray &line : lines) {
                if (line.toLower().startsWith("content-length:"))
                    contentLength = line.mid(15).trimmed().toInt();
            }
            if (buffer->size() < headerEnd + 4 + contentLength)
                return;

            const QByteArray body = buffer->mid(headerEnd + 4, contentLength);
            respond(socket, method, path, body);
            buffer->clear();
        });
        socket->setSocketDescriptor(fd);
    }

private:
    void respond(QTcpSocket *socket, const QString &method,
                 const QString &path, const QByteArray &body)
    {
        QByteArray payload;
        if (method == QLatin1String("GET")
            && path == QLatin1String("/api/workspace/acme")) {
            payload = R"({"workspace":{"id":1,"name":"Acme","slug":"acme","topN":7,"similarityThreshold":0.5,"vectorSearchMode":"default"}})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/setup-complete")) {
            payload = QByteArrayLiteral("{\"results\":{\"VectorDB\":\"") + vectorDb + QByteArrayLiteral("\"}}");
        } else if (method == QLatin1String("GET")
                   && path.startsWith(QLatin1String("/api/system/system-vectors"))) {
            payload = vectorCountBody;
        } else if (method == QLatin1String("POST")
                   && path == QLatin1String("/api/workspace/acme/update")) {
            ++updateRequests;
            lastUpdateBody = body;
            payload = R"({"workspace":{"id":1,"name":"Acme","slug":"acme"},"message":"Workspace updated"})";
        } else {
            payload = R"({"error":"not found"})";
        }

        QByteArray resp = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                          + QByteArray::number(payload.size())
                          + "\r\nConnection: close\r\n\r\n" + payload;
        socket->write(resp);
        socket->flush();
        socket->close();
        socket->deleteLater();
    }
};

class TestVectorDatabaseTab : public QObject
{
    Q_OBJECT

private slots:
    void loadsWorkspaceSearchSettingsAndVectorCount();
    void savesUpdatedSearchFields();
    void searchModeRowHiddenForNonLanceDb();

private:
    void loadTab(VectorDatabaseTab &tab);
};

void TestVectorDatabaseTab::loadTab(VectorDatabaseTab &tab)
{
    tab.setWorkspaceSlug(QStringLiteral("acme"));
    auto *topNSpin = tab.findChild<QSpinBox *>(QStringLiteral("topNSpin"));
    QVERIFY(topNSpin);
    // setLoading(false) runs once the workspace has loaded.
    QTRY_VERIFY_WITH_TIMEOUT(topNSpin->isEnabled(), 5000);
}

void TestVectorDatabaseTab::loadsWorkspaceSearchSettingsAndVectorCount()
{
    MockVectorDbServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    VectorDatabaseTab tab(&client);
    loadTab(tab);

    auto *topNSpin = tab.findChild<QSpinBox *>(QStringLiteral("topNSpin"));
    QCOMPARE(topNSpin->value(), 7);

    auto *thresholdCombo = tab.findChild<QComboBox *>(QStringLiteral("thresholdCombo"));
    QVERIFY(thresholdCombo);
    QCOMPARE(thresholdCombo->currentData().toDouble(), 0.5);

    auto *searchModeRow = tab.findChild<QWidget *>(QStringLiteral("searchModeRow"));
    QVERIFY(searchModeRow);
    QTRY_VERIFY_WITH_TIMEOUT(!searchModeRow->isHidden(), 5000);

    auto *countLabel = tab.findChild<QLabel *>(QStringLiteral("vectorCountLabel"));
    QVERIFY(countLabel);
    QTRY_COMPARE_WITH_TIMEOUT(countLabel->text(), QStringLiteral("42"), 5000);
}

void TestVectorDatabaseTab::savesUpdatedSearchFields()
{
    MockVectorDbServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    VectorDatabaseTab tab(&client);
    loadTab(tab);

    auto *topNSpin = tab.findChild<QSpinBox *>(QStringLiteral("topNSpin"));
    topNSpin->setValue(9);

    auto *saveButton = tab.findChild<QPushButton *>(QStringLiteral("updateWorkspaceButton"));
    QVERIFY(saveButton);
    QTRY_VERIFY_WITH_TIMEOUT(!saveButton->isHidden(), 5000);
    saveButton->click();

    QTRY_COMPARE_WITH_TIMEOUT(server.updateRequests, 1, 5000);
    const QJsonObject sent = QJsonDocument::fromJson(server.lastUpdateBody).object();
    QCOMPARE(sent.value(QStringLiteral("topN")).toInt(), 9);
    QCOMPARE(sent.value(QStringLiteral("similarityThreshold")).toDouble(), 0.5);
}

void TestVectorDatabaseTab::searchModeRowHiddenForNonLanceDb()
{
    MockVectorDbServer server;
    server.vectorDb = "pinecone";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    VectorDatabaseTab tab(&client);
    loadTab(tab);

    auto *searchModeRow = tab.findChild<QWidget *>(QStringLiteral("searchModeRow"));
    QVERIFY(searchModeRow);
    QTRY_VERIFY_WITH_TIMEOUT(searchModeRow->isHidden(), 5000);
}

QTEST_MAIN(TestVectorDatabaseTab)
#include "tst_vector_database_tab.moc"
