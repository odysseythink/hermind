#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QJsonDocument>
#include <QJsonObject>
#include "auth_manager.h"
#include "hermind_api_client.h"
#include "settings_store.h"

class TestAuthManager : public QObject
{
    Q_OBJECT

private slots:
    // skeleton tests from Task 2
    void singletonReturnsSameInstance();
    void initialStateIsUnauthenticated();
    void logoutClearsStateAndEmitsSignals();
    void restoreSessionFromSettings();

    // login tests
    void loginWithSingleUserModeSucceeds();
    void loginWithBadCredentialsFails();

private:
    void waitForSpy(QSignalSpy &spy, int expected);
};

void TestAuthManager::waitForSpy(QSignalSpy &spy, int expected)
{
    int attempts = 0;
    while (spy.count() < expected && attempts < 50) {
        QTest::qWait(100);
        ++attempts;
    }
}

void TestAuthManager::singletonReturnsSameInstance()
{
    AuthManager &a = AuthManager::instance();
    AuthManager &b = AuthManager::instance();
    QCOMPARE(&a, &b);
}

void TestAuthManager::initialStateIsUnauthenticated()
{
    AuthManager &mgr = AuthManager::instance();
    QCOMPARE(mgr.state(), AuthState::Unauthenticated);
    QVERIFY(!mgr.isAuthenticated());
    QVERIFY(mgr.authToken().isEmpty());
    QCOMPARE(mgr.currentUser().id(), 0);
}

void TestAuthManager::logoutClearsStateAndEmitsSignals()
{
    AuthManager &mgr = AuthManager::instance();
    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy tokenSpy(&mgr, &AuthManager::authTokenChanged);
    QSignalSpy userSpy(&mgr, &AuthManager::userChanged);

    mgr.logout();

    QCOMPARE(mgr.state(), AuthState::Unauthenticated);
    QVERIFY(mgr.authToken().isEmpty());
    QCOMPARE(stateSpy.count(), 0); // already Unauthenticated
    QCOMPARE(tokenSpy.count(), 0);
    QCOMPARE(userSpy.count(), 0);
}

void TestAuthManager::restoreSessionFromSettings()
{
    SettingsStore::instance().setAuthToken(QStringLiteral("stored-token"));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(nullptr, &SettingsStore::instance());

    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy tokenSpy(&mgr, &AuthManager::authTokenChanged);

    mgr.restoreSession();

    QCOMPARE(mgr.state(), AuthState::Authenticated);
    QVERIFY(mgr.isAuthenticated());
    QCOMPARE(mgr.authToken(), QStringLiteral("stored-token"));
    QCOMPARE(stateSpy.count(), 1);
    QCOMPARE(tokenSpy.count(), 1);

    // Cleanup
    mgr.logout();
    SettingsStore::instance().setAuthToken(QString());
}

void TestAuthManager::loginWithSingleUserModeSucceeds()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(socket->waitForReadyRead(2000));
        const QByteArray req = socket->readAll();
        QVERIFY(req.contains("POST /api/request-token"));

        const QJsonObject body{{"valid", true}, {"token", "single-user-token"}, {"message", "ok"}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\n");
        socket->write("Content-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n");
        socket->write("\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    });

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());
    SettingsStore::instance().clear();

    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy tokenSpy(&mgr, &AuthManager::authTokenChanged);
    QSignalSpy errorSpy(&mgr, &AuthManager::authError);

    mgr.login();
    waitForSpy(stateSpy, 2); // Authenticating -> Authenticated

    QCOMPARE(mgr.state(), AuthState::Authenticated);
    QCOMPARE(mgr.authToken(), QStringLiteral("single-user-token"));
    QCOMPARE(SettingsStore::instance().authToken(), QStringLiteral("single-user-token"));
    QCOMPARE(stateSpy.count(), 2);
    QCOMPARE(errorSpy.count(), 0);
    QVERIFY(tokenSpy.count() >= 1);

    mgr.logout();
    SettingsStore::instance().clear();
}

void TestAuthManager::loginWithBadCredentialsFails()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(socket->waitForReadyRead(2000));
        const QByteArray req = socket->readAll();
        QVERIFY(req.contains("POST /api/request-token"));

        const QJsonObject body{{"valid", false}, {"token", ""}, {"message", "Invalid credentials"}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\n");
        socket->write("Content-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n");
        socket->write("\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    });

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());
    SettingsStore::instance().clear();

    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy errorSpy(&mgr, &AuthManager::authError);

    mgr.login(QStringLiteral("alice"), QStringLiteral("wrong"));
    waitForSpy(stateSpy, 2); // Authenticating -> Error

    QCOMPARE(mgr.state(), AuthState::Error);
    QCOMPARE(mgr.lastError(), QStringLiteral("Invalid credentials"));
    QCOMPARE(errorSpy.count(), 1);
    QVERIFY(mgr.authToken().isEmpty());

    SettingsStore::instance().clear();
}

QTEST_MAIN(TestAuthManager)
#include "tst_auth_manager.moc"
