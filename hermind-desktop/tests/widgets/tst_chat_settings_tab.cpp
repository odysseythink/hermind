#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QComboBox>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>

#include "chat_settings_tab.h"
#include "hermind_api_client.h"

// Dispatching in-process HTTP server for workspace chat settings.
class MockChatSettingsServer : public QTcpServer
{
    Q_OBJECT
public:
    QByteArray routersBody = R"({"routers":[]})";
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
            payload = R"({"workspace":{"id":1,"name":"Acme","slug":"acme","chatMode":"chat","openAiHistory":20}})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/setup-complete")) {
            payload = R"({"results":{"LLMProvider":"openai"}})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/system/default-system-prompt")) {
            payload = R"({"defaultSystemPrompt":"You are helpful."})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/model-routers")) {
            payload = routersBody;
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

class TestChatSettingsTab : public QObject
{
    Q_OBJECT

private slots:
    void routerProviderShowsEmptyStateWhenNoRouters();
    void routerSelectionPopulatesAndSavesRouterId();

private:
    void loadTab(ChatSettingsTab &tab);
};

void TestChatSettingsTab::loadTab(ChatSettingsTab &tab)
{
    tab.setWorkspaceSlug(QStringLiteral("acme"));
    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("providerCombo"));
    QVERIFY(providerCombo);
    // setLoading(false) runs once workspace + system keys have loaded.
    QTRY_VERIFY_WITH_TIMEOUT(providerCombo->isEnabled(), 5000);
}

void TestChatSettingsTab::routerProviderShowsEmptyStateWhenNoRouters()
{
    MockChatSettingsServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("providerCombo"));
    const int routerIdx = providerCombo->findData(QStringLiteral("anythingllm-router"));
    QVERIFY2(routerIdx >= 0, "anythingllm-router provider must be selectable");
    providerCombo->setCurrentIndex(routerIdx);

    auto *modelCombo = tab.findChild<QComboBox *>(QStringLiteral("modelCombo"));
    auto *modelLineEdit = tab.findChild<QLineEdit *>(QStringLiteral("modelLineEdit"));
    auto *routerRow = tab.findChild<QWidget *>(QStringLiteral("routerSelectionRow"));
    auto *routerCombo = tab.findChild<QComboBox *>(QStringLiteral("routerCombo"));
    auto *emptyLabel = tab.findChild<QLabel *>(QStringLiteral("routerEmptyLabel"));
    QVERIFY(modelCombo);
    QVERIFY(modelLineEdit);
    QVERIFY(routerRow);
    QVERIFY(routerCombo);
    QVERIFY(emptyLabel);

    QVERIFY(modelCombo->isHidden());
    QVERIFY(modelLineEdit->isHidden());
    QVERIFY(!routerRow->isHidden());

    // Backend returned no routers: combo hidden, empty-state label shown.
    QTRY_VERIFY_WITH_TIMEOUT(routerCombo->isHidden() && !emptyLabel->isHidden(), 5000);
}

void TestChatSettingsTab::routerSelectionPopulatesAndSavesRouterId()
{
    MockChatSettingsServer server;
    server.routersBody = R"({"routers":[{"id":"r1","name":"Router A","description":"fast"}]})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("providerCombo"));
    const int routerIdx = providerCombo->findData(QStringLiteral("anythingllm-router"));
    QVERIFY(routerIdx >= 0);
    providerCombo->setCurrentIndex(routerIdx);

    auto *routerCombo = tab.findChild<QComboBox *>(QStringLiteral("routerCombo"));
    QVERIFY(routerCombo);
    QTRY_VERIFY_WITH_TIMEOUT(!routerCombo->isHidden() && routerCombo->count() >= 1, 5000);
    QCOMPARE(routerCombo->itemData(0).toString(), QStringLiteral("r1"));

    auto *saveButton = tab.findChild<QPushButton *>(QStringLiteral("updateWorkspaceButton"));
    QVERIFY(saveButton);
    QTRY_VERIFY_WITH_TIMEOUT(!saveButton->isHidden(), 5000);
    saveButton->click();

    QTRY_COMPARE_WITH_TIMEOUT(server.updateRequests, 1, 5000);
    const QJsonObject sent = QJsonDocument::fromJson(server.lastUpdateBody).object();
    QCOMPARE(sent.value(QStringLiteral("chatProvider")).toString(),
             QStringLiteral("anythingllm-router"));
    QCOMPARE(sent.value(QStringLiteral("router_id")).toString(),
             QStringLiteral("r1"));
    QVERIFY(!sent.contains(QStringLiteral("chatModel")));
}

QTEST_MAIN(TestChatSettingsTab)
#include "tst_chat_settings_tab.moc"
