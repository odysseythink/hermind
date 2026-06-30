#include <QtTest>
#include <QSignalSpy>
#include <QCoreApplication>
#include <QTimer>
#include <iostream>
#include "auth_manager.h"
#include "hermind_api_client.h"
#include "settings_store.h"

class TestAuthManagerLive : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase()
    {
        m_client.setBaseUrl(QUrl(QStringLiteral("http://localhost:3001/api")));
        AuthManager::instance().initialize(&m_client, &SettingsStore::instance());
        SettingsStore::instance().clear();
    }

    void singleUserLoginSucceeds()
    {
        AuthManager &mgr = AuthManager::instance();
        QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
        QSignalSpy errorSpy(&mgr, &AuthManager::authError);

        mgr.login();
        waitForSpy(stateSpy, 2); // Authenticating -> Authenticated

        QCOMPARE(mgr.state(), AuthState::Authenticated);
        QVERIFY(mgr.isAuthenticated());
        QVERIFY(!mgr.authToken().isEmpty());
        QVERIFY(!SettingsStore::instance().authToken().isEmpty());
        QCOMPARE(errorSpy.count(), 0);

        std::cout << "  Token: " << mgr.authToken().toStdString().substr(0, 30) << "..." << std::endl;
    }

    void logoutClearsState()
    {
        AuthManager &mgr = AuthManager::instance();
        QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);

        mgr.logout();

        QCOMPARE(mgr.state(), AuthState::Unauthenticated);
        QVERIFY(!mgr.isAuthenticated());
        QVERIFY(mgr.authToken().isEmpty());
        QVERIFY(SettingsStore::instance().authToken().isEmpty());
    }

    void restoreSessionWorks()
    {
        // First login to get a token persisted to SettingsStore
        AuthManager &mgr = AuthManager::instance();
        {
            QSignalSpy spy(&mgr, &AuthManager::authStateChanged);
            mgr.login();
            waitForSpy(spy, 2);
            QCOMPARE(mgr.state(), AuthState::Authenticated);
        }

        QString storedToken = mgr.authToken();
        QVERIFY(!storedToken.isEmpty());
        QCOMPARE(SettingsStore::instance().authToken(), storedToken);
        std::cout << "  Token persisted: " << storedToken.toStdString().substr(0, 30) << "..." << std::endl;

        // Simulate restart: reset AuthManager state without clearing store
        // (logout clears the store, so we manually reset internal state instead)
        mgr.setAuthToken(QString());
        mgr.setUser(HermindUser());
        mgr.setState(AuthState::Unauthenticated);
        // But SettingsStore still has the token
        QCOMPARE(SettingsStore::instance().authToken(), storedToken);

        // restoreSession should read from SettingsStore and re-authenticate
        QSignalSpy stateSpy(&mgr, &AuthManager::authStateChanged);
        mgr.restoreSession();
        // restoreSession sets state to Authenticated synchronously
        QCOMPARE(mgr.state(), AuthState::Authenticated);
        QCOMPARE(mgr.authToken(), storedToken);

        // Cleanup
        mgr.logout();
        SettingsStore::instance().clear();
    }

    void cleanupTestCase()
    {
        AuthManager::instance().logout();
        SettingsStore::instance().clear();
    }

private:
    void waitForSpy(QSignalSpy &spy, int expected)
    {
        int attempts = 0;
        while (spy.count() < expected && attempts < 100) {
            QTest::qWait(100);
            ++attempts;
        }
    }

    HermindApiClient m_client;
};

QTEST_MAIN(TestAuthManagerLive)
#include "tst_auth_manager_live.moc"
