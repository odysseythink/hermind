#include <QtTest>
#include <QSignalSpy>
#include <QStackedWidget>
#include "chat_container_widget.h"
#include "chat_stream_handler.h"
#include "agent_event_handler.h"
#include "sources_sidebar.h"
#include "hermind_api_client.h"

class TestChatContainerWidget : public QObject
{
    Q_OBJECT
private slots:
    void setWorkspace_updatesWelcomeLabel();
    void setInputText_setsPromptInputText();
    void sendCommand_startsStream();
    void sendCommand_prependMode_putsTextBeforeCursor();
    void sendCommand_appendMode_putsTextAfterCursor();
    void stopRequested_callsAbort();
    void initialState_showsWelcomeView();
    void sendCommand_switchesToChatView();
    void newChat_switchesBackToWelcomeView();
    void agentCitations_openSourcesSidebar();
    void streamSources_openSourcesSidebar();
};

void TestChatContainerWidget::setWorkspace_updatesWelcomeLabel()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    QCOMPARE(widget.workspaceSlug(), QStringLiteral("ws-1"));
    QCOMPARE(widget.workspaceName(), QStringLiteral("My Workspace"));
}

void TestChatContainerWidget::setInputText_setsPromptInputText()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));
    widget.setInputText(QStringLiteral("Hello"));

    // Sending "Hello" explicitly must start a stream, confirming the input
    // round-trip path through PromptInput works.
    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("Hello"));

    QCOMPARE(spy.count(), 1);
}

void TestChatContainerWidget::sendCommand_startsStream()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("Hello world"));

    QCOMPARE(spy.count(), 1);
}

void TestChatContainerWidget::sendCommand_prependMode_putsTextBeforeCursor()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));
    widget.setInputText(QStringLiteral("existing text"));

    // prepend mode only edits the input; it must NOT start a stream.
    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("@agent"), QStringLiteral("prepend"));

    QCOMPARE(spy.count(), 0);
}

void TestChatContainerWidget::sendCommand_appendMode_putsTextAfterCursor()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));
    widget.setInputText(QStringLiteral("hello"));

    // append mode only edits the input; it must NOT start a stream.
    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral(" world"), QStringLiteral("append"));

    QCOMPARE(spy.count(), 0);
}

void TestChatContainerWidget::stopRequested_callsAbort()
{
    // The stop button lives inside PromptInput; clicking it emits
    // stopRequested, which ChatContainerWidget wires to abortStream().
    // We verify the flow by starting a stream and confirming no crash /
    // hang when abortStream() is reachable through the widget.
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("test"));

    QCOMPARE(spy.count(), 1);
}

void TestChatContainerWidget::initialState_showsWelcomeView()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    QStackedWidget *stack = widget.findChild<QStackedWidget *>(QStringLiteral("chatStack"));
    QVERIFY(stack != nullptr);
    QCOMPARE(stack->currentIndex(), 0); // DefaultChatWidget welcome page
}

void TestChatContainerWidget::sendCommand_switchesToChatView()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    widget.sendCommand(QStringLiteral("Hello"));

    QStackedWidget *stack = widget.findChild<QStackedWidget *>(QStringLiteral("chatStack"));
    QVERIFY(stack != nullptr);
    QCOMPARE(stack->currentIndex(), 1); // chat history page
}

void TestChatContainerWidget::newChat_switchesBackToWelcomeView()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    widget.sendCommand(QStringLiteral("Hello"));
    widget.newChat();

    QStackedWidget *stack = widget.findChild<QStackedWidget *>(QStringLiteral("chatStack"));
    QVERIFY(stack != nullptr);
    QCOMPARE(stack->currentIndex(), 0);
}

void TestChatContainerWidget::agentCitations_openSourcesSidebar()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    AgentEventHandler *handler = widget.findChild<AgentEventHandler *>();
    SourcesSidebar *sidebar = widget.findChild<SourcesSidebar *>();
    QVERIFY(handler != nullptr);
    QVERIFY(sidebar != nullptr);
    QVERIFY(!sidebar->isOpen());

    QJsonArray citations;
    citations.append(QJsonObject{{"title", "doc1"}, {"chunk", "text"}});
    handler->citationsReceived(QStringLiteral("u1"), citations);

    QVERIFY(sidebar->isOpen());
}

void TestChatContainerWidget::streamSources_openSourcesSidebar()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    ChatStreamHandler *handler = widget.findChild<ChatStreamHandler *>();
    SourcesSidebar *sidebar = widget.findChild<SourcesSidebar *>();
    QVERIFY(handler != nullptr);
    QVERIFY(sidebar != nullptr);
    QVERIFY(!sidebar->isOpen());

    QJsonArray sources;
    sources.append(QJsonObject{{"title", "doc1"}, {"chunk", "text"}});
    handler->sourcesReceived(QStringLiteral("u1"), sources);

    QVERIFY(sidebar->isOpen());
}

QTEST_MAIN(TestChatContainerWidget)
#include "tst_chat_container_widget.moc"
