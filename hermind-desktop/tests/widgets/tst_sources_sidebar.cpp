#include <QtTest>
#include <QSignalSpy>
#include "sources_sidebar.h"
#include "source_item.h"

class TestSourcesSidebar : public QObject
{
    Q_OBJECT
private slots:
    void startsClosed();
    void openShowsSidebar();
    void closeHidesSidebar();
    void setSourcesPopulatesItems();
    void setSourcesDeduplicatesByTitle();
};

void TestSourcesSidebar::startsClosed()
{
    SourcesSidebar sidebar;
    QVERIFY(!sidebar.isOpen());
}

void TestSourcesSidebar::openShowsSidebar()
{
    SourcesSidebar sidebar;
    sidebar.open();
    QVERIFY(sidebar.isOpen());
}

void TestSourcesSidebar::closeHidesSidebar()
{
    SourcesSidebar sidebar;
    sidebar.open();
    QSignalSpy spy(&sidebar, &SourcesSidebar::closeRequested);
    sidebar.close();
    QVERIFY(!sidebar.isOpen());
    QCOMPARE(spy.count(), 1);
}

void TestSourcesSidebar::setSourcesPopulatesItems()
{
    SourcesSidebar sidebar;
    QJsonArray arr;
    QJsonObject src1;
    src1.insert(QStringLiteral("title"), QStringLiteral("Doc A"));
    src1.insert(QStringLiteral("description"), QStringLiteral("Description A"));
    QJsonObject src2;
    src2.insert(QStringLiteral("title"), QStringLiteral("Doc B"));
    src2.insert(QStringLiteral("description"), QStringLiteral("Description B"));
    arr.append(src1);
    arr.append(src2);

    sidebar.setSources(arr);
    sidebar.open();

    QList<SourceItem *> items = sidebar.findChildren<SourceItem *>();
    QCOMPARE(items.count(), 2);
}

void TestSourcesSidebar::setSourcesDeduplicatesByTitle()
{
    SourcesSidebar sidebar;
    QJsonArray arr;
    QJsonObject src1;
    src1.insert(QStringLiteral("title"), QStringLiteral("Doc A"));
    src1.insert(QStringLiteral("description"), QStringLiteral("Description A"));
    QJsonObject src2;
    src2.insert(QStringLiteral("title"), QStringLiteral("Doc B"));
    src2.insert(QStringLiteral("description"), QStringLiteral("Description B"));
    QJsonObject src3;
    src3.insert(QStringLiteral("title"), QStringLiteral("Doc A"));
    src3.insert(QStringLiteral("description"), QStringLiteral("Duplicate chunk of Doc A"));
    arr.append(src1);
    arr.append(src2);
    arr.append(src3);

    sidebar.setSources(arr);
    sidebar.open();

    QList<SourceItem *> items = sidebar.findChildren<SourceItem *>();
    QCOMPARE(items.count(), 2);
}

QTEST_MAIN(TestSourcesSidebar)
#include "tst_sources_sidebar.moc"
