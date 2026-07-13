#include <QtTest>
#include "hermind_memory.h"

class TestHermindMemory : public QObject
{
    Q_OBJECT

private slots:
    void roundTripJson();
    void fromJsonParsesAllFields();
};

void TestHermindMemory::roundTripJson()
{
    HermindMemory m;
    m.setId(42);
    m.setScope(QStringLiteral("workspace"));
    m.setContent(QStringLiteral("Remember X"));

    QJsonObject obj = m.toJson();
    HermindMemory m2 = HermindMemory::fromJson(obj);

    QCOMPARE(m2.id(), 42);
    QCOMPARE(m2.scope(), QStringLiteral("workspace"));
    QCOMPARE(m2.content(), QStringLiteral("Remember X"));
}

void TestHermindMemory::fromJsonParsesAllFields()
{
    QJsonObject obj;
    obj.insert(QStringLiteral("id"), 1);
    obj.insert(QStringLiteral("scope"), QStringLiteral("global"));
    obj.insert(QStringLiteral("content"), QStringLiteral("Test memory"));
    obj.insert(QStringLiteral("createdAt"), QStringLiteral("2026-07-01T00:00:00"));

    HermindMemory m = HermindMemory::fromJson(obj);
    QCOMPARE(m.id(), 1);
    QCOMPARE(m.scope(), QStringLiteral("global"));
    QCOMPARE(m.content(), QStringLiteral("Test memory"));
    QVERIFY(m.createdAt().isValid());
}

QTEST_MAIN(TestHermindMemory)
#include "tst_hermind_memory.moc"
