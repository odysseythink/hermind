#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QLineEdit>
#include <QLabel>
#include <QPushButton>
#include <QDialogButtonBox>
#include <QJsonObject>
#include <QJsonDocument>
#include "new_workspace_dialog.h"
#include "hermind_api_client.h"
#include "hermind_workspace.h"

class TestNewWorkspaceDialog : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void hasRequiredChildren();
    void saveButtonDisabledWhenNameEmpty();
    void saveButtonEnabledAfterTyping();
    void createsWorkspaceViaApi();
    void showsErrorOnApiFailure();

private:
    class MockHttpServer;
    MockHttpServer *m_server = nullptr;
    HermindApiClient *m_client = nullptr;
};

class TestNewWorkspaceDialog::MockHttpServer : public QTcpServer
{
    Q_OBJECT
public:
    using Handler = std::function<QByteArray(const QString &method,
                                             const QString &path,
                                             const QByteArray &body)>;
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
            if (lines.size() < 2)
                lines = text.split(QStringLiteral("\n"));

            if (lines.isEmpty()) {
                socket->close();
                socket->deleteLater();
                return;
            }

            QStringList parts = lines.first().split(' ');
            QString method = parts.value(0);
            QString encodedPath = parts.value(1);
            QString path = QUrl::fromPercentEncoding(encodedPath.toUtf8());

            QByteArray body;
            int blank = data.indexOf("\r\n\r\n");
            int step = 4;
            if (blank < 0) {
                blank = data.indexOf("\n\n");
                step = 2;
            }
            if (blank >= 0)
                body = data.mid(blank + step);

            QByteArray respBody = m_handler ? m_handler(method, path, body) : QByteArray("{}");
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

void TestNewWorkspaceDialog::initTestCase()
{
    m_server = new MockHttpServer(this);
    QVERIFY(m_server->start());

    m_client = new HermindApiClient(this);
    m_client->setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(m_server->port())));
    m_client->setAuthToken(QStringLiteral("jwt"));
}

void TestNewWorkspaceDialog::cleanupTestCase()
{
    delete m_client;
    delete m_server;
}

void TestNewWorkspaceDialog::hasRequiredChildren()
{
    NewWorkspaceDialog dialog;
    QVERIFY(dialog.findChild<QLineEdit *>());
    QVERIFY(dialog.findChild<QDialogButtonBox *>());
    QVERIFY(dialog.findChild<QLabel *>(QStringLiteral("errorLabel")));
}

void TestNewWorkspaceDialog::saveButtonDisabledWhenNameEmpty()
{
    NewWorkspaceDialog dialog;
    QPushButton *saveButton = dialog.findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save);
    QVERIFY(saveButton);
    QVERIFY(!saveButton->isEnabled());
}

void TestNewWorkspaceDialog::saveButtonEnabledAfterTyping()
{
    NewWorkspaceDialog dialog;
    QLineEdit *nameEdit = dialog.findChild<QLineEdit *>();
    QPushButton *saveButton = dialog.findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save);
    QVERIFY(nameEdit);
    QVERIFY(saveButton);

    nameEdit->setText(QStringLiteral("My Workspace"));
    QVERIFY(saveButton->isEnabled());

    nameEdit->clear();
    QVERIFY(!saveButton->isEnabled());
}

void TestNewWorkspaceDialog::createsWorkspaceViaApi()
{
    QByteArray lastBody;
    m_server->setHandler([&lastBody](const QString &method, const QString &path,
                                     const QByteArray &body) -> QByteArray {
        lastBody = body;
        if (method == QStringLiteral("POST") && path == QStringLiteral("/api/workspace/new")) {
            return QByteArray(R"({"workspace":{"id":3,"name":"My Workspace","slug":"my-workspace"},"message":"Workspace created"})");
        }
        return QByteArray("{}");
    });

    NewWorkspaceDialog dialog;
    dialog.setApiClient(m_client);

    QLineEdit *nameEdit = dialog.findChild<QLineEdit *>();
    QPushButton *saveButton = dialog.findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save);
    QVERIFY(nameEdit);
    QVERIFY(saveButton);

    QSignalSpy createdSpy(&dialog, &NewWorkspaceDialog::workspaceCreated);
    nameEdit->setText(QStringLiteral("My Workspace"));
    QVERIFY(saveButton->isEnabled());

    QTest::mouseClick(saveButton, Qt::LeftButton);
    QVERIFY(createdSpy.wait(5000));
    QCOMPARE(createdSpy.count(), 1);

    const HermindWorkspace workspace = createdSpy.first().first().value<HermindWorkspace>();
    QCOMPARE(workspace.name(), QStringLiteral("My Workspace"));
    QCOMPARE(workspace.slug(), QStringLiteral("my-workspace"));

    QJsonObject expectedBody;
    expectedBody.insert(QStringLiteral("name"), QStringLiteral("My Workspace"));
    QCOMPARE(lastBody, QJsonDocument(expectedBody).toJson(QJsonDocument::Compact));
}

void TestNewWorkspaceDialog::showsErrorOnApiFailure()
{
    m_server->setHandler([](const QString &, const QString &, const QByteArray &) -> QByteArray {
        return QByteArray(R"({"workspace":null,"error":"Name already in use"})");
    });

    NewWorkspaceDialog dialog;
    dialog.setApiClient(m_client);

    QLineEdit *nameEdit = dialog.findChild<QLineEdit *>();
    QPushButton *saveButton = dialog.findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save);
    QLabel *errorLabel = dialog.findChild<QLabel *>(QStringLiteral("errorLabel"));
    QVERIFY(errorLabel);

    nameEdit->setText(QStringLiteral("Duplicate"));
    dialog.show();
    QVERIFY(QTest::qWaitForWindowExposed(&dialog));
    QTest::mouseClick(saveButton, Qt::LeftButton);

    QTRY_VERIFY_WITH_TIMEOUT(errorLabel->isVisible(), 5000);
    QVERIFY(errorLabel->text().contains(QStringLiteral("Name already in use")));
}

QTEST_MAIN(TestNewWorkspaceDialog)
#include "tst_new_workspace_dialog.moc"
