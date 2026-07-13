#include <QtTest>
#include "memories_sidebar.h"
#include "hermind_api_client.h"

class TestMemoriesSidebar : public QObject
{
    Q_OBJECT

private slots:
    void startsClosed();
    void openShowsAndCloseHides();
};

void TestMemoriesSidebar::startsClosed()
{
    HermindApiClient client;
    MemoriesSidebar sidebar(&client);
    QVERIFY(!sidebar.isOpen());
}

void TestMemoriesSidebar::openShowsAndCloseHides()
{
    HermindApiClient client;
    MemoriesSidebar sidebar(&client);
    sidebar.open();
    QVERIFY(sidebar.isOpen());
    sidebar.close();
    QVERIFY(!sidebar.isOpen());
}

QTEST_MAIN(TestMemoriesSidebar)
#include "tst_memories_sidebar.moc"
