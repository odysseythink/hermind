#include <QtTest>
#include "auth_state.h"

class TestAuthState : public QObject
{
    Q_OBJECT

private slots:
    void enumValuesExist();
    void authResultDefaults();
    void authResultCanHoldUser();
};

void TestAuthState::enumValuesExist()
{
    QCOMPARE(static_cast<int>(AuthState::Unauthenticated), 0);
    QCOMPARE(static_cast<int>(AuthState::Authenticating), 1);
    QCOMPARE(static_cast<int>(AuthState::Authenticated), 2);
    QCOMPARE(static_cast<int>(AuthState::Error), 3);
}

void TestAuthState::authResultDefaults()
{
    AuthResult r;
    QVERIFY(!r.success);
    QVERIFY(r.token.isEmpty());
    QVERIFY(r.message.isEmpty());
    QCOMPARE(r.user.id(), 0);
}

void TestAuthState::authResultCanHoldUser()
{
    QJsonObject obj;
    obj.insert("id", 42);
    obj.insert("username", "alice");
    obj.insert("role", "admin");
    obj.insert("suspended", 0);

    AuthResult r;
    r.success = true;
    r.token = "jwt-abc";
    r.user = HermindUser::fromJson(obj);
    r.message = "ok";

    QVERIFY(r.success);
    QCOMPARE(r.token, QStringLiteral("jwt-abc"));
    QCOMPARE(r.user.username(), QStringLiteral("alice"));
    QCOMPARE(r.user.role(), QStringLiteral("admin"));
    QCOMPARE(r.message, QStringLiteral("ok"));
}

QTEST_MAIN(TestAuthState)
#include "tst_auth_state.moc"
