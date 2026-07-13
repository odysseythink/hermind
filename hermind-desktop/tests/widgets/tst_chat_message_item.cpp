#include <QtTest>
#include <QApplication>
#include "chat_message_item.h"
#include "hermind_chat_message.h"

class TestChatMessageItem : public QObject
{
    Q_OBJECT
private slots:
    void userMessage_alignsRight();
    void assistantMessage_alignsLeft();
};

void TestChatMessageItem::userMessage_alignsRight()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent("Hello");

    ChatMessageItem item(nullptr);
    item.setMessage(msg);

    QCOMPARE(item.messageText(), QString("Hello"));
    QVERIFY(item.isUserMessage());
}

void TestChatMessageItem::assistantMessage_alignsLeft()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::Assistant);
    msg.setContent("Hi there");

    ChatMessageItem item(nullptr);
    item.setMessage(msg);

    QVERIFY(!item.isUserMessage());
}

QTEST_MAIN(TestChatMessageItem)
#include "tst_chat_message_item.moc"
