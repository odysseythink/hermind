#include <QtTest>
#include <QApplication>
#include <QLabel>
#include <QTextBrowser>

#include "markdown_message_item.h"
#include "plain_message_item.h"
#include "hermind_chat_message.h"

class TestMarkdownMessageItem : public QObject
{
    Q_OBJECT
private slots:
    void streamingMessage_showsPlainText();
    void closedMessage_usesRenderer();
    void streamThenClose_switchesToMarkdown();
    void plainItem_showsText();
};

static HermindChatMessage makeAssistantMessage(const QString &content, bool closed)
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::Assistant);
    msg.setContent(content);
    msg.setClosed(closed);
    return msg;
}

void TestMarkdownMessageItem::streamingMessage_showsPlainText()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("partial **bold**"), false));

    QVERIFY(item.findChild<QLabel *>() != nullptr);
    QVERIFY(item.findChild<QTextBrowser *>() == nullptr);
}

void TestMarkdownMessageItem::closedMessage_usesRenderer()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("**bold text**"), true));

    QTextBrowser *browser = item.findChild<QTextBrowser *>();
    QVERIFY(browser != nullptr);
    QVERIFY(browser->toPlainText().contains(QStringLiteral("bold text")));
}

void TestMarkdownMessageItem::streamThenClose_switchesToMarkdown()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("partial **bold text**"), false));
    QVERIFY(item.findChild<QLabel *>() != nullptr);
    QVERIFY(item.findChild<QTextBrowser *>() == nullptr);

    item.setMessage(makeAssistantMessage(QStringLiteral("**bold text**"), true));
    QTextBrowser *browser = item.findChild<QTextBrowser *>();
    QVERIFY(browser != nullptr);
    QVERIFY(browser->toPlainText().contains(QStringLiteral("bold text")));
}

void TestMarkdownMessageItem::plainItem_showsText()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent(QStringLiteral("Hello"));

    PlainMessageItem item(nullptr);
    item.setMessage(msg);

    QCOMPARE(item.messageText(), QStringLiteral("Hello"));
}

QTEST_MAIN(TestMarkdownMessageItem)
#include "tst_markdown_message_item.moc"
