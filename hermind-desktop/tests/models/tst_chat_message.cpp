#include <QtTest>
#include "hermind_chat_message.h"

class TestChatMessage : public QObject
{
    Q_OBJECT
private slots:
    void fromJson_populatesAllFields();
    void updateAppendsText_preservesUuidAndRole();
};

void TestChatMessage::fromJson_populatesAllFields()
{
    QJsonObject obj;
    obj.insert("id", 42);
    obj.insert("uuid", "uuid-1");
    obj.insert("role", "assistant");
    obj.insert("user", "user-1");
    obj.insert("content", "Hello");
    obj.insert("sentAt", 1700000000);
    obj.insert("createdAt", 1700000001);
    obj.insert("feedbackScore", 1);
    obj.insert("chatMode", "chat");
    obj.insert("close", true);

    HermindChatMessage msg = HermindChatMessage::fromJson(obj);

    QCOMPARE(msg.id(), 42);
    QCOMPARE(msg.uuid(), QString("uuid-1"));
    QCOMPARE(msg.role(), HermindChatMessage::Assistant);
    QCOMPARE(msg.user(), QString("user-1"));
    QCOMPARE(msg.content(), QString("Hello"));
    QCOMPARE(msg.sentAt().toSecsSinceEpoch(), qint64(1700000000));
    QCOMPARE(msg.feedbackScore(), 1);
    QCOMPARE(msg.chatMode(), QString("chat"));
    QVERIFY(msg.isClosed());
}

void TestChatMessage::updateAppendsText_preservesUuidAndRole()
{
    HermindChatMessage msg = HermindChatMessage::fromJson(QJsonObject{
        {"uuid", "uuid-2"}, {"role", "assistant"}, {"content", "Hel"}
    });
    msg.appendContent("lo");

    QCOMPARE(msg.uuid(), QString("uuid-2"));
    QCOMPARE(msg.role(), HermindChatMessage::Assistant);
    QCOMPARE(msg.content(), QString("Hello"));
}

QTEST_APPLESS_MAIN(TestChatMessage)
#include "tst_chat_message.moc"
