#include <QtTest>
#include <QPushButton>
#include "suggested_messages.h"

class TestSuggestedMessages : public QObject
{
    Q_OBJECT
private slots:
    void setMessagesCreatesButtons();
    void clickingButtonInvokesCallback();
};

void TestSuggestedMessages::setMessagesCreatesButtons()
{
    SuggestedMessages widget;
    widget.setMessages({ QStringLiteral("Msg 1"), QStringLiteral("Msg 2") });

    QList<QPushButton *> btns = widget.findChildren<QPushButton *>();
    // 2 message buttons
    QCOMPARE(btns.count(), 2);
}

void TestSuggestedMessages::clickingButtonInvokesCallback()
{
    SuggestedMessages widget;
    QString captured;
    widget.setSendCommandCallback([&](const QString &text, const QString &) {
        captured = text;
    });
    widget.setMessages({ QStringLiteral("Click me") });

    QList<QPushButton *> btns = widget.findChildren<QPushButton *>();
    QCOMPARE(btns.count(), 1);
    QTest::mouseClick(btns.first(), Qt::LeftButton);

    QCOMPARE(captured, QStringLiteral("Click me"));
}

QTEST_MAIN(TestSuggestedMessages)
#include "tst_suggested_messages.moc"
