#include <QtTest>
#include <QApplication>
#include <QClipboard>
#include <QLabel>
#include <QPushButton>
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
    void copyButton_copiesContentToClipboard();
    void plainItem_copyButton_copiesContent();
    void regenerateButton_emitsSignal();
    void streamingMessage_hidesRegenerateButton();
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

void TestMarkdownMessageItem::copyButton_copiesContentToClipboard()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("copy me"), true));

    QPushButton *copyBtn = item.findChild<QPushButton *>(QStringLiteral("copyBtn"));
    QVERIFY(copyBtn != nullptr);

    // Retry: the Windows clipboard is a global resource and can be briefly
    // locked by another test process when suites run back-to-back.
    QTRY_VERIFY_WITH_TIMEOUT((copyBtn->click(),
        QGuiApplication::clipboard()->text() == QStringLiteral("copy me")), 5000);
}

void TestMarkdownMessageItem::plainItem_copyButton_copiesContent()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent(QStringLiteral("user copy text"));

    PlainMessageItem item(nullptr);
    item.setMessage(msg);

    QPushButton *copyBtn = item.findChild<QPushButton *>(QStringLiteral("copyBtn"));
    QVERIFY(copyBtn != nullptr);

    QTRY_VERIFY_WITH_TIMEOUT((copyBtn->click(),
        QGuiApplication::clipboard()->text() == QStringLiteral("user copy text")), 5000);
}

void TestMarkdownMessageItem::regenerateButton_emitsSignal()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("answer"), true));

    QPushButton *regenBtn = item.findChild<QPushButton *>(QStringLiteral("regenerateBtn"));
    QVERIFY(regenBtn != nullptr);
    QVERIFY(!regenBtn->isHidden());

    QSignalSpy spy(&item, &MarkdownMessageItem::regenerateRequested);
    regenBtn->click();

    QCOMPARE(spy.count(), 1);
}

void TestMarkdownMessageItem::streamingMessage_hidesRegenerateButton()
{
    MarkdownMessageItem item(nullptr);
    item.setMessage(makeAssistantMessage(QStringLiteral("partial"), false));

    QPushButton *regenBtn = item.findChild<QPushButton *>(QStringLiteral("regenerateBtn"));
    if (regenBtn)
        QVERIFY(regenBtn->isHidden());
    // Absence of the button is also acceptable while streaming.
}

QTEST_MAIN(TestMarkdownMessageItem)
#include "tst_markdown_message_item.moc"
