#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include "thread_container_widget.h"
#include "thread_item_widget.h"
#include "hermind_api_client.h"

class TestThreadContainerWidget : public QObject
{
    Q_OBJECT
private slots:
    void initTestCase();
    void cleanupTestCase();
    void loadsThreadsFromApi();
    void clickThreadEmitsThreadClicked();

private:
    class MockHttpServer;
    MockHttpServer *m_server = nullptr;
    HermindApiClient *m_client = nullptr;
};

class TestThreadContainerWidget::MockHttpServer : public QTcpServer
{
    Q_OBJECT
public:
    using Handler = std::function<QByteArray(const QString &method, const QString &path)>;
    explicit MockHttpServer(QObject *parent = nullptr) : QTcpServer(parent) {}
    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }
    void setHandler(Handler h) { m_handler = std::move(h); }

protected:
    void incomingConnection(qintptr socketDescriptor) override
    {
        QTcpSocket *socket = new QTcpSocket(this);
        connect(socket, &QTcpSocket::readyRead, this, [this, socket]() {
            QByteArray data = socket->readAll();
            QString text = QString::fromUtf8(data);
            QStringList lines = text.split(QStringLiteral("\r\n"));
            if (lines.isEmpty()) { socket->close(); socket->deleteLater(); return; }
            QStringList parts = lines.first().split(' ');
            QString method = parts.value(0);
            QString encodedPath = parts.value(1);
            QString path = QUrl::fromPercentEncoding(encodedPath.toUtf8());
            QByteArray respBody = m_handler ? m_handler(method, path) : QByteArray("{}");
            QByteArray response = "HTTP/1.1 200 OK\r\n"
                                  "Content-Type: application/json\r\n"
                                  "Content-Length: " + QByteArray::number(respBody.size()) +
                                  "\r\nConnection: close\r\n\r\n" + respBody;
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

void TestThreadContainerWidget::initTestCase()
{
    m_server = new MockHttpServer(this);
    QVERIFY(m_server->start());
    m_client = new HermindApiClient(this);
    m_client->setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(m_server->port())));
    m_client->setAuthToken(QStringLiteral("jwt"));
}

void TestThreadContainerWidget::cleanupTestCase()
{
    delete m_client;
    delete m_server;
}

void TestThreadContainerWidget::loadsThreadsFromApi()
{
    m_server->setHandler([](const QString &, const QString &) {
        return QByteArray(R"({"threads":[{"id":1,"name":"Thread A","slug":"thread-a","workspaceId":1},{"id":2,"name":"Thread B","slug":"thread-b","workspaceId":1}]})");
    });

    ThreadContainerWidget widget;
    widget.setApiClient(m_client);
    widget.setWorkspaceSlug(QStringLiteral("default"));

    QTRY_VERIFY_WITH_TIMEOUT(widget.findChildren<ThreadItemWidget *>().size() == 3, 5000); // default + 2 + new button is not ThreadItemWidget
    const auto items = widget.findChildren<ThreadItemWidget *>();
    QCOMPARE(items.size(), 3);
    QCOMPARE(items.at(0)->threadSlug(), QString());
    QCOMPARE(items.at(1)->threadSlug(), QStringLiteral("thread-a"));
    QCOMPARE(items.at(2)->threadSlug(), QStringLiteral("thread-b"));
}

void TestThreadContainerWidget::clickThreadEmitsThreadClicked()
{
    m_server->setHandler([](const QString &, const QString &) {
        return QByteArray(R"({"threads":[{"id":1,"name":"Thread A","slug":"thread-a","workspaceId":1}]})");
    });

    ThreadContainerWidget widget;
    widget.setApiClient(m_client);
    widget.setWorkspaceSlug(QStringLiteral("default"));

    QTRY_VERIFY_WITH_TIMEOUT(widget.findChildren<ThreadItemWidget *>().size() == 2, 5000);
    auto *threadItem = widget.findChildren<ThreadItemWidget *>().at(1);

    QSignalSpy spy(&widget, &ThreadContainerWidget::threadClicked);
    threadItem->simulateClick();
    QCOMPARE(spy.count(), 1);
    const QList<QVariant> args = spy.takeFirst();
    QCOMPARE(args.at(0).toString(), QStringLiteral("default"));
    QCOMPARE(args.at(1).toString(), QStringLiteral("thread-a"));
}

QTEST_MAIN(TestThreadContainerWidget)
#include "tst_thread_container_widget.moc"
