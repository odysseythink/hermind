#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include "active_workspaces_widget.h"
#include "workspace_item_widget.h"
#include "hermind_api_client.h"
#include "navigation_manager.h"

class TestActiveWorkspacesWidget : public QObject
{
    Q_OBJECT
private slots:
    void initTestCase();
    void cleanupTestCase();
    void loadsWorkspacesFromApi();
    void clickNavigatesToWorkspaceChat();

private:
    class MockHttpServer;
    MockHttpServer *m_server = nullptr;
    HermindApiClient *m_client = nullptr;
};

class TestActiveWorkspacesWidget::MockHttpServer : public QTcpServer
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

void TestActiveWorkspacesWidget::initTestCase()
{
    m_server = new MockHttpServer(this);
    QVERIFY(m_server->start());
    m_client = new HermindApiClient(this);
    m_client->setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(m_server->port())));
    m_client->setAuthToken(QStringLiteral("jwt"));

    // 重置导航历史，避免之前测试的副作用
    NavigationManager::instance().clearHistory();
}

void TestActiveWorkspacesWidget::cleanupTestCase()
{
    delete m_client;
    delete m_server;
}

void TestActiveWorkspacesWidget::loadsWorkspacesFromApi()
{
    m_server->setHandler([](const QString &, const QString &) {
        return QByteArray(R"({"workspaces":[{"id":1,"name":"Default","slug":"default","openAiHistory":20},{"id":2,"name":"KB","slug":"kb","openAiHistory":20}]})");
    });

    ActiveWorkspacesWidget widget;
    widget.setApiClient(m_client);
    widget.refresh();

    QTRY_VERIFY_WITH_TIMEOUT(widget.findChildren<WorkspaceItemWidget *>().size() == 2, 5000);
    const auto items = widget.findChildren<WorkspaceItemWidget *>();
    QCOMPARE(items.size(), 2);
    QCOMPARE(items.at(0)->workspaceSlug(), QStringLiteral("default"));
    QCOMPARE(items.at(1)->workspaceSlug(), QStringLiteral("kb"));
}

void TestActiveWorkspacesWidget::clickNavigatesToWorkspaceChat()
{
    m_server->setHandler([](const QString &, const QString &) {
        return QByteArray(R"({"workspaces":[{"id":1,"name":"Default","slug":"default","openAiHistory":20}]})");
    });

    ActiveWorkspacesWidget widget;
    widget.setApiClient(m_client);
    widget.refresh();
    QTRY_VERIFY_WITH_TIMEOUT(widget.findChildren<WorkspaceItemWidget *>().size() == 1, 5000);

    QSignalSpy navSpy(&NavigationManager::instance(), &NavigationManager::currentRouteChanged);
    widget.findChildren<WorkspaceItemWidget *>().first()->simulateClick();

    QTRY_VERIFY_WITH_TIMEOUT(!navSpy.isEmpty(), 1000);
    const NavigationRoute route = NavigationManager::instance().currentRoute();
    QCOMPARE(route.page, NavigationPage::WorkspaceChat);
    QCOMPARE(route.workspaceSlug, QStringLiteral("default"));
}

QTEST_MAIN(TestActiveWorkspacesWidget)
#include "tst_active_workspaces_widget.moc"
