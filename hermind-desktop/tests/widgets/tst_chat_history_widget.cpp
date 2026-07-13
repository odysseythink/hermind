#include <QtTest>
#include <QScrollArea>
#include <QScrollBar>
#include "chat_history_widget.h"
#include "hermind_chat_message.h"
#include "plain_message_item.h"
#include "markdown_message_item.h"

class TestChatHistoryWidget : public QObject
{
    Q_OBJECT
private slots:
    void setMessages_createsItems();
    void appendMessage_scrollsToBottom();
    void userAndAssistant_createDifferentItemTypes();
    void appendMessage_whenScrolledUp_doesNotAutoScroll();
};

void TestChatHistoryWidget::setMessages_createsItems()
{
    ChatHistoryWidget widget(nullptr);

    QVector<HermindChatMessage> msgs;
    HermindChatMessage m1;
    m1.setRole(HermindChatMessage::User);
    m1.setContent("Hi");
    msgs.append(m1);

    HermindChatMessage m2;
    m2.setRole(HermindChatMessage::Assistant);
    m2.setContent("Hello");
    msgs.append(m2);

    widget.setMessages(msgs);

    QCOMPARE(widget.messageCount(), 2);
}

void TestChatHistoryWidget::appendMessage_scrollsToBottom()
{
    ChatHistoryWidget widget(nullptr);
    widget.appendMessage([]{
        HermindChatMessage m;
        m.setRole(HermindChatMessage::Assistant);
        m.setContent("Bottom");
        return m;
    }());

    QCOMPARE(widget.messageCount(), 1);
    QVERIFY(widget.isAtBottom());
}

void TestChatHistoryWidget::userAndAssistant_createDifferentItemTypes()
{
    ChatHistoryWidget widget(nullptr);

    QVector<HermindChatMessage> msgs;
    HermindChatMessage userMsg;
    userMsg.setRole(HermindChatMessage::User);
    userMsg.setContent("Question");
    msgs.append(userMsg);

    HermindChatMessage assistantMsg;
    assistantMsg.setRole(HermindChatMessage::Assistant);
    assistantMsg.setContent("Answer");
    msgs.append(assistantMsg);

    widget.setMessages(msgs);

    QVERIFY(widget.findChild<PlainMessageItem *>() != nullptr);
    QVERIFY(widget.findChild<MarkdownMessageItem *>() != nullptr);
}

void TestChatHistoryWidget::appendMessage_whenScrolledUp_doesNotAutoScroll()
{
    ChatHistoryWidget widget(nullptr);
    widget.resize(300, 150);
    widget.show();

    auto makeMsg = [](const QString &text) {
        HermindChatMessage m;
        m.setRole(HermindChatMessage::User);
        m.setContent(text);
        return m;
    };

    QVector<HermindChatMessage> bulk;
    for (int i = 0; i < 60; ++i)
        bulk.append(makeMsg(QStringLiteral("filler line %1").arg(i)));
    widget.setMessages(bulk);

    QScrollBar *bar = widget.findChild<QScrollArea *>()->verticalScrollBar();
    QTRY_VERIFY_WITH_TIMEOUT(bar->maximum() > 0, 5000);

    bar->setValue(0);
    QCOMPARE(bar->value(), 0);

    widget.appendMessage(makeMsg(QStringLiteral("new message while scrolled up")));
    QTest::qWait(100); // give any (erroneous) auto-scroll a chance to run

    QCOMPARE(bar->value(), 0);
}

QTEST_MAIN(TestChatHistoryWidget)
#include "tst_chat_history_widget.moc"
