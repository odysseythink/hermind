#include <QtTest>
#include <QSignalSpy>
#include "thread_item_widget.h"
#include "hermind_workspace_thread.h"

class TestThreadItemWidget : public QObject
{
    Q_OBJECT
private slots:
    void displaysThreadName();
    void defaultThreadShowsDefaultLabel();
    void clickEmitsThreadClicked();
    void selectedStateStyle();
};

void TestThreadItemWidget::displaysThreadName()
{
    ThreadItemWidget item;
    HermindWorkspaceThread thread = HermindWorkspaceThread::fromJson(QJsonObject{
        {"id", 1}, {"name", "Planning"}, {"slug", "planning"}, {"workspaceId", 1}
    });
    item.setWorkspaceSlug(QStringLiteral("ws"));
    item.setThread(thread);
    QCOMPARE(item.workspaceSlug(), QStringLiteral("ws"));
    QCOMPARE(item.threadSlug(), QStringLiteral("planning"));
}

void TestThreadItemWidget::defaultThreadShowsDefaultLabel()
{
    ThreadItemWidget item;
    item.setWorkspaceSlug(QStringLiteral("ws"));
    item.setDefaultThread(true);
    QVERIFY(item.isDefaultThread());
    QCOMPARE(item.threadSlug(), QString());
}

void TestThreadItemWidget::clickEmitsThreadClicked()
{
    ThreadItemWidget item;
    HermindWorkspaceThread thread = HermindWorkspaceThread::fromJson(QJsonObject{
        {"id", 1}, {"name", "Planning"}, {"slug", "planning"}, {"workspaceId", 1}
    });
    item.setWorkspaceSlug(QStringLiteral("ws"));
    item.setThread(thread);

    QSignalSpy spy(&item, &ThreadItemWidget::threadClicked);
    item.simulateClick();
    QCOMPARE(spy.count(), 1);
    const QList<QVariant> args = spy.takeFirst();
    QCOMPARE(args.at(0).toString(), QStringLiteral("ws"));
    QCOMPARE(args.at(1).toString(), QStringLiteral("planning"));
}

void TestThreadItemWidget::selectedStateStyle()
{
    ThreadItemWidget item;
    item.setWorkspaceSlug(QStringLiteral("ws"));
    item.setDefaultThread(true);
    QVERIFY(!item.isSelected());
    item.setSelected(true);
    QVERIFY(item.isSelected());
}

QTEST_MAIN(TestThreadItemWidget)
#include "tst_thread_item_widget.moc"
