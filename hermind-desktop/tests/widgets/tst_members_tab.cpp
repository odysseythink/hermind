#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonArray>
#include <QListWidget>
#include <QPushButton>
#include <QTableWidget>
#include <QSignalSpy>

#include "members_tab.h"
#include "hermind_api_client.h"

// Dispatching in-process HTTP server: routes by method+path and captures
// the last update-users request body.
class MockMembersServer : public QTcpServer
{
    Q_OBJECT
public:
    int workspaceUsersRequests = 0;
    QByteArray lastUpdateBody;
    int updateRequests = 0;
    // Users currently reported as workspace members (mutable per test).
    QByteArray workspaceUsersBody;
    QByteArray allUsersBody;

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
            QString path = QString::fromUtf8(requestLine.at(1));

            int contentLength = 0;
            for (const QByteArray &line : lines) {
                if (line.toLower().startsWith("content-length:"))
                    contentLength = line.mid(15).trimmed().toInt();
            }
            if (buffer->size() < headerEnd + 4 + contentLength)
                return; // body not fully received yet

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
            && path.startsWith(QLatin1String("/api/workspace/"))) {
            payload = R"({"workspace":{"id":1,"name":"Acme","slug":"acme"}})";
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/admin/workspaces/1/users")) {
            ++workspaceUsersRequests;
            payload = workspaceUsersBody;
        } else if (method == QLatin1String("GET")
                   && path == QLatin1String("/api/admin/users")) {
            payload = allUsersBody;
        } else if (method == QLatin1String("POST")
                   && path == QLatin1String("/api/admin/workspaces/1/update-users")) {
            ++updateRequests;
            lastUpdateBody = body;
            payload = R"({"success":true})";
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

class TestMembersTab : public QObject
{
    Q_OBJECT

private slots:
    void loadsMembersForWorkspace();
    void emptyWorkspaceShowsPlaceholder();
    void manageDialogFiltersAdminsAndSavesSelection();
};

static QByteArray usersBody(std::initializer_list<const char *> members)
{
    QJsonArray arr;
    int id = 100;
    for (const char *name : members) {
        QJsonObject u;
        u.insert(QStringLiteral("userId"), id++);
        u.insert(QStringLiteral("username"), QString::fromUtf8(name));
        u.insert(QStringLiteral("role"), QStringLiteral("default"));
        u.insert(QStringLiteral("lastUpdatedAt"), QStringLiteral("2026-01-02T00:00:00Z"));
        arr.append(u);
    }
    QJsonObject obj;
    obj.insert(QStringLiteral("users"), arr);
    return QJsonDocument(obj).toJson(QJsonDocument::Compact);
}

void TestMembersTab::loadsMembersForWorkspace()
{
    MockMembersServer server;
    server.workspaceUsersBody = usersBody({"bob", "dave"});
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    MembersTab tab(&client);
    QSignalSpy spy(&tab, &MembersTab::membersLoaded);

    tab.setWorkspaceSlug(QStringLiteral("acme"));

    QTRY_COMPARE_WITH_TIMEOUT(spy.count(), 1, 5000);
    QCOMPARE(spy.takeFirst().at(0).toInt(), 2);

    auto *table = tab.findChild<QTableWidget *>(QStringLiteral("membersTable"));
    QVERIFY(table);
    QCOMPARE(table->rowCount(), 2);
    QCOMPARE(table->item(0, 0)->text(), QStringLiteral("bob"));
    QCOMPARE(table->item(1, 0)->text(), QStringLiteral("dave"));

    auto *manage = tab.findChild<QPushButton *>(QStringLiteral("manageMembersButton"));
    QVERIFY(manage);
    QVERIFY(manage->isEnabled());
}

void TestMembersTab::emptyWorkspaceShowsPlaceholder()
{
    MockMembersServer server;
    server.workspaceUsersBody = usersBody({});
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    MembersTab tab(&client);
    QSignalSpy spy(&tab, &MembersTab::membersLoaded);

    tab.setWorkspaceSlug(QStringLiteral("acme"));

    QTRY_COMPARE_WITH_TIMEOUT(spy.count(), 1, 5000);
    QCOMPARE(spy.takeFirst().at(0).toInt(), 0);

    auto *table = tab.findChild<QTableWidget *>(QStringLiteral("membersTable"));
    QVERIFY(table);
    QCOMPARE(table->rowCount(), 1);
    QCOMPARE(table->item(0, 0)->text(), QStringLiteral("No workspace members"));
}

void TestMembersTab::manageDialogFiltersAdminsAndSavesSelection()
{
    MockMembersServer server;
    // bob (userId 100) is a current member.
    server.workspaceUsersBody = usersBody({"bob"});
    // All users: admin + manager must be excluded from the dialog.
    server.allUsersBody = R"({"users":[
        {"id":1,"username":"root","role":"admin","suspended":0},
        {"id":2,"username":"boss","role":"manager","suspended":0},
        {"id":100,"username":"bob","role":"default","suspended":0},
        {"id":200,"username":"dave","role":"default","suspended":0}
    ]})";
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    MembersTab tab(&client);
    QSignalSpy spy(&tab, &MembersTab::membersLoaded);
    tab.setWorkspaceSlug(QStringLiteral("acme"));
    QTRY_COMPARE_WITH_TIMEOUT(spy.count(), 1, 5000);
    spy.clear();

    auto *manage = tab.findChild<QPushButton *>(QStringLiteral("manageMembersButton"));
    QVERIFY(manage);
    manage->click();

    auto *dialog = tab.findChild<ManageMembersDialog *>(
        QStringLiteral("manageMembersDialog"));
    QVERIFY(dialog);

    auto *list = dialog->findChild<QListWidget *>(QStringLiteral("memberUserList"));
    QVERIFY(list);
    // Only bob and dave are selectable; admin/manager are filtered out.
    QTRY_COMPARE_WITH_TIMEOUT(list->count(), 2, 5000);

    QCOMPARE(list->item(0)->text(), QStringLiteral("bob"));
    QCOMPARE(list->item(0)->checkState(), Qt::Checked);   // current member
    QCOMPARE(list->item(1)->text(), QStringLiteral("dave"));
    QCOMPARE(list->item(1)->checkState(), Qt::Unchecked);

    // Add dave.
    list->item(1)->setCheckState(Qt::Checked);

    auto *save = dialog->findChild<QPushButton *>(QStringLiteral("saveMembersButton"));
    QVERIFY(save);
    save->click();

    QTRY_COMPARE_WITH_TIMEOUT(server.updateRequests, 1, 5000);

    const QJsonObject sent = QJsonDocument::fromJson(server.lastUpdateBody).object();
    const QJsonArray ids = sent.value(QStringLiteral("userIds")).toArray();
    QCOMPARE(ids.size(), 2);
    QCOMPARE(ids.at(0).toInt(), 100);
    QCOMPARE(ids.at(1).toInt(), 200);

    // After saving, the tab reloads members.
    QTRY_COMPARE_WITH_TIMEOUT(server.workspaceUsersRequests, 2, 5000);
}

QTEST_MAIN(TestMembersTab)
#include "tst_members_tab.moc"
