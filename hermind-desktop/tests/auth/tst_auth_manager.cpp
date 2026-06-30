#include <QtTest>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QJsonDocument>
#include <QJsonObject>
#include <QCoreApplication>
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

    // refreshUser tests
    void refreshUserSingleUserModeKeepsEmptyUser();
    void refreshUserMultiUserModeUpdatesUser();
    void refreshUserInvalidSessionLogsOut();

    // logout test
    void logoutClearsStoredTokenAndClientToken();

private:
    void waitForSpy(QSignalSpy &spy, int expected);
    bool waitForSocketReady(QTcpSocket *socket, int timeoutMs = 5000);
};

void TestAuthManager::waitForSpy(QSignalSpy &spy, int expected)
{
    int attempts = 0;
    while (spy.count() < expected && attempts < 100) {
        QTest::qWait(100);
        ++attempts;
    }
}

bool TestAuthManager::waitForSocketReady(QTcpSocket *socket, int timeoutMs)
{
    int elapsed = 0;
    while (elapsed < timeoutMs) {
        QTest::qWait(50);
        elapsed += 50;
        if (socket->bytesAvailable() > 0)
            return true;
        if (socket->state() != QAbstractSocket::ConnectedState)
            break;
    }
    return socket->bytesAvailable() > 0;
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
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(waitForSocketReady(socket));
        socket->readAll();

        const QJsonObject body{{"success", true}, {"user", QJsonValue::Null}, {"message", QJsonValue::Null}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    }, Qt::QueuedConnection);

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    SettingsStore::instance().setAuthToken(QStringLiteral("stored-token"));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());

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
    server.close();
    QTest::qWait(10);
}

void TestAuthManager::loginWithSingleUserModeSucceeds()
{
    QCoreApplication::processEvents();
    QTest::qWait(50);
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(waitForSocketReady(socket));
        socket->readAll(); // consume request

        const QJsonObject body{{"valid", true}, {"token", "single-user-token"}, {"message", "ok"}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\n");
        socket->write("Content-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n");
        socket->write("\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    }, Qt::QueuedConnection);

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
        QVERIFY(waitForSocketReady(socket));
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
    }, Qt::QueuedConnection);

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

void TestAuthManager::refreshUserSingleUserModeKeepsEmptyUser()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(waitForSocketReady(socket));
        socket->readAll();

        const QJsonObject body{{"success", true}, {"user", QJsonValue::Null}, {"message", QJsonValue::Null}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    }, Qt::QueuedConnection);

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());
    SettingsStore::instance().setAuthToken(QStringLiteral("token"));

    QSignalSpy userSpy(&mgr, &AuthManager::userChanged);
    QSignalSpy errorSpy(&mgr, &AuthManager::authError);

    mgr.restoreSession();
    waitForSpy(userSpy, 1);

    QCOMPARE(mgr.state(), AuthState::Authenticated);
    QCOMPARE(mgr.currentUser().id(), 0);
    QCOMPARE(errorSpy.count(), 0);

    mgr.logout();
    SettingsStore::instance().clear();
}

void TestAuthManager::refreshUserMultiUserModeUpdatesUser()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(waitForSocketReady(socket));
        socket->readAll();

        QJsonObject user{{"id", 7}, {"username", "bob"}, {"role", "manager"}, {"suspended", 0}};
        const QJsonObject body{{"success", true}, {"user", user}, {"message", QJsonValue::Null}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    }, Qt::QueuedConnection);

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());
    SettingsStore::instance().setAuthToken(QStringLiteral("token"));

    QSignalSpy userSpy(&mgr, &AuthManager::userChanged);

    mgr.restoreSession();
    waitForSpy(userSpy, 1);

    QCOMPARE(mgr.currentUser().id(), 7);
    QCOMPARE(mgr.currentUser().username(), QStringLiteral("bob"));
    QCOMPARE(mgr.currentUser().role(), QStringLiteral("manager"));

    mgr.logout();
    SettingsStore::instance().clear();
}

void TestAuthManager::refreshUserInvalidSessionLogsOut()
{
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost, 0));
    const quint16 port = server.serverPort();

    connect(&server, &QTcpServer::newConnection, &server, [&]() {
        QTcpSocket *socket = server.nextPendingConnection();
        QVERIFY(waitForSocketReady(socket));
        socket->readAll();

        const QJsonObject body{{"success", false}, {"user", QJsonValue::Null}, {"message", "Session expired or invalid."}};
        const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);
        socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n");
        socket->write("Content-Length: " + QByteArray::number(payload.size()) + "\r\n\r\n");
        socket->write(payload);
        socket->flush();
        socket->close();
    }, Qt::QueuedConnection);

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(port)));

    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());
    SettingsStore::instance().setAuthToken(QStringLiteral("token"));

    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy errorSpy(&mgr, &AuthManager::authError);

    mgr.restoreSession();
    waitForSpy(stateSpy, 2); // Authenticated -> Unauthenticated

    QCOMPARE(mgr.state(), AuthState::Unauthenticated);
    QVERIFY(mgr.authToken().isEmpty());
    QCOMPARE(errorSpy.count(), 1);

    SettingsStore::instance().clear();
}

void TestAuthManager::logoutClearsStoredTokenAndClientToken()
{
    HermindApiClient client;
    AuthManager &mgr = AuthManager::instance();
    mgr.initialize(&client, &SettingsStore::instance());

    // Set up authenticated-like state so logout transition is observable
    SettingsStore::instance().setAuthToken(QStringLiteral("to-be-cleared"));
    mgr.setAuthToken(QStringLiteral("to-be-cleared"));
    client.setAuthToken(QStringLiteral("to-be-cleared"));
    // Set an actual user to trigger userChanged on logout
    QJsonObject userObj{{"id", 1}, {"username", "test"}, {"role", "default"}, {"suspended", 0}};
    mgr.setUser(HermindUser::fromJson(userObj));

    QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
    QSignalSpy tokenSpy(&mgr, &AuthManager::authTokenChanged);
    QSignalSpy userSpy(&mgr, &AuthManager::userChanged);

    mgr.logout();

    QCOMPARE(mgr.state(), AuthState::Unauthenticated);
    QVERIFY(mgr.authToken().isEmpty());
    QVERIFY(SettingsStore::instance().authToken().isEmpty());
    QVERIFY(client.authToken().isEmpty());
    QCOMPARE(mgr.currentUser().id(), 0);

    QCOMPARE(stateSpy.count(), 0); // already Unauthenticated from prior tests
    QCOMPARE(tokenSpy.count(), 1);
    QCOMPARE(userSpy.count(), 1); // user was non-empty, so logout clears it

    SettingsStore::instance().clear();
}

QTEST_MAIN(TestAuthManager)
#include "tst_auth_manager.moc"
