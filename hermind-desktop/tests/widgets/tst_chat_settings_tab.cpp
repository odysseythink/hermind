#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QComboBox>
#include <QDialog>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QTextEdit>

#include "chat_settings_tab.h"
#include "hermind_api_client.h"

// Dispatching in-process HTTP server for workspace chat settings.
class MockChatSettingsServer : public QTcpServer
{
    Q_OBJECT
public:
    QByteArray routersBody = R"({"routers":[]})";
    QByteArray variablesBody = R"({"variables":[]})";
    QByteArray historyBody = R"({"history":[]})";
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
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/system/prompt-variables")) {
            payload = variablesBody;
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/workspace/acme/prompt-history")) {
            payload = historyBody;
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
    void modeExplanationFollowsSelectedChatMode();
    void promptVariablesHintShowsAvailableVariables();
    void promptVariablesHintHiddenWhenNoVariables();
    void promptHistoryDialogRestoresPromptIntoEditor();

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

void TestChatSettingsTab::modeExplanationFollowsSelectedChatMode()
{
    MockChatSettingsServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *explanation = tab.findChild<QLabel *>(QStringLiteral("chatModeExplanation"));
    QVERIFY2(explanation, "chat mode explanation label must exist");

    // Workspace loads with chatMode "chat".
    QVERIFY(explanation->text().contains(QStringLiteral("general knowledge")));

    auto *queryBtn = tab.findChild<QPushButton *>(QStringLiteral("modeButton_query"));
    QVERIFY(queryBtn);
    queryBtn->click();
    QVERIFY(explanation->text().contains(QStringLiteral("only if document context")));

    auto *agentBtn = tab.findChild<QPushButton *>(QStringLiteral("modeButton_automatic"));
    QVERIFY(agentBtn);
    agentBtn->click();
    QVERIFY(explanation->text().contains(QStringLiteral("automatically use tools")));
}

void TestChatSettingsTab::promptVariablesHintShowsAvailableVariables()
{
    MockChatSettingsServer server;
    server.variablesBody = R"({"variables":[{"key":"foo"},{"key":"bar"},{"key":"baz"},{"key":"qux"}]})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *hint = tab.findChild<QLabel *>(QStringLiteral("promptVariablesHint"));
    QVERIFY2(hint, "prompt variables hint label must exist");
    QTRY_VERIFY_WITH_TIMEOUT(!hint->isHidden(), 5000);
    QVERIFY(hint->text().contains(QStringLiteral("{foo}")));
    QVERIFY(hint->text().contains(QStringLiteral("{bar}")));
    QVERIFY(hint->text().contains(QStringLiteral("{baz}")));
    QVERIFY(hint->text().contains(QStringLiteral("+1 more")));
}

void TestChatSettingsTab::promptVariablesHintHiddenWhenNoVariables()
{
    MockChatSettingsServer server;
    server.variablesBody = R"({"variables":[]})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *hint = tab.findChild<QLabel *>(QStringLiteral("promptVariablesHint"));
    QVERIFY2(hint, "prompt variables hint label must exist");
    // Give the variables request time to resolve; hint must stay hidden.
    QTest::qWait(500);
    QVERIFY(hint->isHidden());
}

void TestChatSettingsTab::promptHistoryDialogRestoresPromptIntoEditor()
{
    MockChatSettingsServer server;
    server.historyBody = R"({"history":[{"id":1,"workspaceId":1,"prompt":"restored prompt text","modifiedAt":"2026-07-01T00:00:00Z"}]})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatSettingsTab tab(&client);
    loadTab(tab);

    auto *historyButton = tab.findChild<QPushButton *>(QStringLiteral("promptHistoryButton"));
    QVERIFY2(historyButton, "prompt history button must exist");
    historyButton->click();

    QDialog *dialog = nullptr;
    QTRY_VERIFY_WITH_TIMEOUT(
        (dialog = tab.findChild<QDialog *>(QStringLiteral("promptHistoryDialog"))) != nullptr, 5000);

    QPushButton *restoreButton = nullptr;
    QTRY_VERIFY_WITH_TIMEOUT(
        (restoreButton = dialog->findChild<QPushButton *>(QStringLiteral("restorePromptButton_1"))) != nullptr,
        5000);
    restoreButton->click();

    auto *promptEdit = tab.findChild<QTextEdit *>(QStringLiteral("promptEdit"));
    QVERIFY(promptEdit);
    QCOMPARE(promptEdit->toPlainText(), QStringLiteral("restored prompt text"));

    auto *saveButton = tab.findChild<QPushButton *>(QStringLiteral("updateWorkspaceButton"));
    QVERIFY(saveButton);
    QVERIFY(!saveButton->isHidden());
}

QTEST_MAIN(TestChatSettingsTab)
#include "tst_chat_settings_tab.moc"
