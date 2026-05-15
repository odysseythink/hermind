#include <QTest>
#include "../src/httplib.h"

class TestHttpLib : public QObject
{
    Q_OBJECT
private slots:
    void testBaseUrl();
};

void TestHttpLib::testBaseUrl()
{
    HermindClient client("http://127.0.0.1:12345");
    QCOMPARE(client.baseUrl(), QString("http://127.0.0.1:12345"));
}

QTEST_MAIN(TestHttpLib)
#include "test_httplib.moc"
