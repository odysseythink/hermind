#include <QtTest>
#include <QSignalSpy>
#include "auth_manager.h"
#include "settings_store.h"

class TestAuthManager : public QObject
{
    Q_OBJECT

private slots:
    void singletonReturnsSameInstance();
    void initialStateIsUnauthenticated();
    void logoutClearsStateAndEmitsSignals();
    void restoreSessionFromSettings();
};

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

QTEST_MAIN(TestAuthManager)
#include "tst_auth_manager.moc"
