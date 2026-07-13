#include <QtTest>
#include <QTcpServer>
#include <QTcpSocket>
#include "memories_sidebar.h"
#include "memory_card.h"
#include "hermind_api_client.h"

// Minimal in-process HTTP server: counts requests and returns a fixed
// memory list payload.
class MockMemoryServer : public QTcpServer
{
    Q_OBJECT
public:
    int requests = 0;

    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }

protected:
    void incomingConnection(qintptr fd) override
    {
        auto *socket = new QTcpSocket(this);
        connect(socket, &QTcpSocket::readyRead, this, [this, socket]() {
            socket->readAll();
            ++requests;
            QByteArray body = R"({"workspace":[{"id":1,"content":"remember this","scope":"workspace"}],"global":[]})";
            QByteArray resp = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                              + QByteArray::number(body.size())
                              + "\r\nConnection: close\r\n\r\n" + body;
            socket->write(resp);
            socket->flush();
            socket->close();
            socket->deleteLater();
        });
        socket->setSocketDescriptor(fd);
    }
};

class TestMemoriesSidebar : public QObject
{
    Q_OBJECT

private slots:
    void startsClosed();
    void openShowsAndCloseHides();
    void open_fetchesMemoriesForCurrentWorkspace();
};

void TestMemoriesSidebar::startsClosed()
{
    HermindApiClient client;
    MemoriesSidebar sidebar(&client);
    QVERIFY(!sidebar.isOpen());
}

void TestMemoriesSidebar::openShowsAndCloseHides()
{
    HermindApiClient client;
    MemoriesSidebar sidebar(&client);
    sidebar.open();
    QVERIFY(sidebar.isOpen());
    sidebar.close();
    QVERIFY(!sidebar.isOpen());
}

// Regression: opening the panel must fetch memories. Previously fetch only
// happened when setWorkspace() was called while already open (or after a
// mutation), so the first open always showed an empty list.
void TestMemoriesSidebar::open_fetchesMemoriesForCurrentWorkspace()
{
    MockMemoryServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    MemoriesSidebar sidebar(&client);
    sidebar.setWorkspace(QStringLiteral("ws1"), 1);
    QCOMPARE(server.requests, 0); // closed: no fetch yet

    sidebar.open();
    QTRY_COMPARE(server.requests, 1);
    QTRY_COMPARE(sidebar.findChildren<MemoryCard *>().size(), 1);
}

QTEST_MAIN(TestMemoriesSidebar)
#include "tst_memories_sidebar.moc"
