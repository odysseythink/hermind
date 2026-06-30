#include <QtTest>
#include <QCoreApplication>
#include "hermind_api_client.h"
#include "hermind_user.h"
#include "hermind_workspace.h"

class TestBackendLive : public QObject
{
    Q_OBJECT

private slots:
    void requestTokenAndRefreshUserAndWorkspaces();
};

void TestBackendLive::requestTokenAndRefreshUserAndWorkspaces()
{
    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://localhost:3001/api")));

    // 1. request-token (single-user mode: empty username/password)
    bool tokenDone = false;
    QString token;
    ApiError tokenErr;
    client.requestToken(QString(), QString(),
                        [&](const QString &t, const QString &, const ApiError &e) {
                            tokenDone = true;
                            token = t;
                            tokenErr = e;
                        });
    QTRY_VERIFY_WITH_TIMEOUT(tokenDone, 10000);
    QVERIFY2(tokenErr.isEmpty(), qPrintable(tokenErr.message()));
    QVERIFY(!token.isEmpty());

    client.setAuthToken(token);

    // 2. refresh-user
    bool userDone = false;
    HermindUser user;
    ApiError userErr;
    client.refreshUser([&](const HermindUser &u, const QString &, const ApiError &e) {
        userDone = true;
        user = u;
        userErr = e;
    });
    QTRY_VERIFY_WITH_TIMEOUT(userDone, 10000);
    QVERIFY2(userErr.isEmpty(), qPrintable(userErr.message()));

    // 3. list workspaces
    bool wsDone = false;
    QVector<HermindWorkspace> workspaces;
    ApiError wsErr;
    client.listWorkspaces([&](const QVector<HermindWorkspace> &list, const ApiError &e) {
        wsDone = true;
        workspaces = list;
        wsErr = e;
    });
    QTRY_VERIFY_WITH_TIMEOUT(wsDone, 10000);
    QVERIFY2(wsErr.isEmpty(), qPrintable(wsErr.message()));
    QVERIFY(workspaces.size() >= 0);
}

QTEST_MAIN(TestBackendLive)
#include "tst_backend_live.moc"
