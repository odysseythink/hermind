#include <QtTest>
#include <QSignalSpy>
#include <QPushButton>
#include "tool_approval_dialog.h"

class TestToolApprovalDialog : public QObject
{
    Q_OBJECT
private slots:
    void approveEmitsApproved();
    void rejectEmitsRejected();
};

void TestToolApprovalDialog::approveEmitsApproved()
{
    ToolApprovalDialog dlg(QStringLiteral("req-1"),
                           QStringLiteral("web_search"),
                           QStringLiteral("Search the web for X"));
    QSignalSpy spy(&dlg, &ToolApprovalDialog::approved);

    QList<QPushButton *> btns = dlg.findChildren<QPushButton *>();
    QPushButton *btn = nullptr;
    for (auto *b : btns) {
        if (b->text() == QStringLiteral("批准")) { btn = b; break; }
    }
    QVERIFY(btn != nullptr);
    QTest::mouseClick(btn, Qt::LeftButton);

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).toString(), QStringLiteral("req-1"));
}

void TestToolApprovalDialog::rejectEmitsRejected()
{
    ToolApprovalDialog dlg(QStringLiteral("req-2"),
                           QStringLiteral("run_script"),
                           QStringLiteral("Execute script.sh"));
    QSignalSpy spy(&dlg, &ToolApprovalDialog::rejected);

    QList<QPushButton *> btns = dlg.findChildren<QPushButton *>();
    QPushButton *btn = nullptr;
    for (auto *b : btns) {
        if (b->text() == QStringLiteral("拒绝")) { btn = b; break; }
    }
    QVERIFY(btn != nullptr);
    QTest::mouseClick(btn, Qt::LeftButton);

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).toString(), QStringLiteral("req-2"));
}

QTEST_MAIN(TestToolApprovalDialog)
#include "tst_tool_approval_dialog.moc"
