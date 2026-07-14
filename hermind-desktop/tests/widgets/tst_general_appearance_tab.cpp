#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonArray>
#include <QLineEdit>
#include <QPushButton>
#include <QSignalSpy>

#include "general_appearance_tab.h"
#include "suggested_messages_editor.h"
#include "hermind_api_client.h"

// Dispatching in-process HTTP server for workspace settings endpoints.
class MockGeneralServer : public QTcpServer
{
    Q_OBJECT
public:
    int updateRequests = 0;
    QByteArray lastUpdateBody;
    QByteArray setupResultsBody = R"({"results":{}})";

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
            payload = R"({"workspace":{"id":1,"name":"Acme","slug":"acme"}})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/workspace/acme/suggested-messages")) {
            payload = R"({"suggestedMessages":["Hello","How are you?"]})";
        } else if (method == QLatin1String("POST")
                   && path == QLatin1String("/api/workspace/acme/update")) {
            ++updateRequests;
            lastUpdateBody = body;
            const QJsonObject sent = QJsonDocument::fromJson(body).object();
            const QString name = sent.value(QStringLiteral("name")).toString();
            payload = QByteArray(R"({"workspace":{"id":1,"name":")")
                      + name.toUtf8()
                      + QByteArray(R"(","slug":"acme"},"message":"updated"})");
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/setup-complete")) {
            payload = setupResultsBody;
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

class TestGeneralAppearanceTab : public QObject
{
    Q_OBJECT

private slots:
    void loadsWorkspaceNameAndSuggestedMessages();
    void updateNamePostsNameField();
    void deleteRowVisibleWhenNotProtected();
    void deletionProtectionHidesDeleteRow();
};

void TestGeneralAppearanceTab::loadsWorkspaceNameAndSuggestedMessages()
{
    MockGeneralServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    GeneralAppearanceTab tab(&client);
    tab.setWorkspaceSlug(QStringLiteral("acme"));

    auto *nameEdit = tab.findChild<QLineEdit *>(QStringLiteral("workspaceNameEdit"));
    QVERIFY(nameEdit);
    QTRY_COMPARE_WITH_TIMEOUT(nameEdit->text(), QStringLiteral("Acme"), 5000);

    auto *update = tab.findChild<QPushButton *>(QStringLiteral("updateNameButton"));
    QVERIFY(update);
    QVERIFY(update->isEnabled());

    auto *editor = tab.findChild<SuggestedMessagesEditor *>(
        QStringLiteral("suggestedMessagesEditor"));
    QVERIFY(editor);
    QTRY_COMPARE_WITH_TIMEOUT(editor->validMessages().size(), 2, 5000);
}

void TestGeneralAppearanceTab::updateNamePostsNameField()
{
    MockGeneralServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    GeneralAppearanceTab tab(&client);
    tab.setWorkspaceSlug(QStringLiteral("acme"));

    auto *nameEdit = tab.findChild<QLineEdit *>(QStringLiteral("workspaceNameEdit"));
    QVERIFY(nameEdit);
    QTRY_COMPARE_WITH_TIMEOUT(nameEdit->text(), QStringLiteral("Acme"), 5000);

    QSignalSpy spy(&tab, &GeneralAppearanceTab::workspaceUpdated);
    nameEdit->setText(QStringLiteral("Acme Renamed"));
    auto *update = tab.findChild<QPushButton *>(QStringLiteral("updateNameButton"));
    update->click();

    QTRY_COMPARE_WITH_TIMEOUT(server.updateRequests, 1, 5000);
    const QJsonObject sent = QJsonDocument::fromJson(server.lastUpdateBody).object();
    QCOMPARE(sent.value(QStringLiteral("name")).toString(),
             QStringLiteral("Acme Renamed"));

    QTRY_COMPARE_WITH_TIMEOUT(spy.count(), 1, 5000);
}

void TestGeneralAppearanceTab::deleteRowVisibleWhenNotProtected()
{
    MockGeneralServer server;
    server.setupResultsBody = R"({"results":{}})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    GeneralAppearanceTab tab(&client);
    tab.setWorkspaceSlug(QStringLiteral("acme"));

    auto *del = tab.findChild<QPushButton *>(QStringLiteral("deleteWorkspaceButton"));
    QVERIFY(del);
    QTRY_VERIFY_WITH_TIMEOUT(del->isEnabled(), 5000);
    auto *row = tab.findChild<QWidget *>(QStringLiteral("deleteWorkspaceRow"));
    QVERIFY(row);
    QVERIFY(!row->isHidden());
}

void TestGeneralAppearanceTab::deletionProtectionHidesDeleteRow()
{
    MockGeneralServer server;
    server.setupResultsBody = R"({"results":{"WorkspaceDeletionProtection":true}})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    GeneralAppearanceTab tab(&client);
    tab.setWorkspaceSlug(QStringLiteral("acme"));

    auto *nameEdit = tab.findChild<QLineEdit *>(QStringLiteral("workspaceNameEdit"));
    QVERIFY(nameEdit);
    QTRY_COMPARE_WITH_TIMEOUT(nameEdit->text(), QStringLiteral("Acme"), 5000);

    auto *del = tab.findChild<QPushButton *>(QStringLiteral("deleteWorkspaceButton"));
    QVERIFY(del);
    auto *row = tab.findChild<QWidget *>(QStringLiteral("deleteWorkspaceRow"));
    QVERIFY(row);
    QTRY_VERIFY_WITH_TIMEOUT(row->isHidden(), 5000);
}

QTEST_MAIN(TestGeneralAppearanceTab)
#include "tst_general_appearance_tab.moc"
