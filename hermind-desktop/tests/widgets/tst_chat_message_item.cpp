#include <QtTest>
#include <QApplication>
#include <QHBoxLayout>
#include <QFrame>
#include "plain_message_item.h"
#include "hermind_chat_message.h"

class TestChatMessageItem : public QObject
{
    Q_OBJECT
private slots:
    void userMessage_alignsRight();
    void bubble_alignsRightInOuterLayout();
};

void TestChatMessageItem::userMessage_alignsRight()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent("Hello");

    PlainMessageItem item(nullptr);
    item.setMessage(msg);

    QCOMPARE(item.messageText(), QString("Hello"));
}

void TestChatMessageItem::bubble_alignsRightInOuterLayout()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent("Hello");

    PlainMessageItem item(nullptr);
    item.setMessage(msg);

    QFrame *bubble = item.findChild<QFrame *>(QStringLiteral("bubbleFrame"));
    QVERIFY(bubble != nullptr);

    auto *outer = qobject_cast<QHBoxLayout *>(item.layout());
    QVERIFY(outer != nullptr);
    QVERIFY(outer->alignment() & Qt::AlignRight);
}

QTEST_MAIN(TestChatMessageItem)
#include "tst_chat_message_item.moc"
