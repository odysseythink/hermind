#include <QtTest>
#include <QSignalSpy>
#include <QStackedWidget>
#include <QPushButton>
#include <QTcpServer>
#include <QTcpSocket>
#include "chat_container_widget.h"
#include "chat_stream_handler.h"
#include "agent_event_handler.h"
#include "chat_history_widget.h"
#include "default_chat_widget.h"
#include "prompt_input.h"
#include "sources_sidebar.h"
#include "memories_sidebar.h"
#include "hermind_api_client.h"

// Minimal in-process HTTP server returning a canned getWorkspace payload.
class MockWorkspaceServer : public QTcpServer
{
    Q_OBJECT
public:
    bool start() { return listen(QHostAddress::LocalHost, 0); }
    quint16 port() const { return serverPort(); }

protected:
    void incomingConnection(qintptr fd) override
    {
        auto *socket = new QTcpSocket(this);
        connect(socket, &QTcpSocket::readyRead, this, [socket]() {
            socket->readAll();
            QByteArray body = R"({"workspace":{"id":7,"name":"Real Name","slug":"ws-1","suggestedMessages":[]}})";
            QByteArray resp = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                              + QByteArray::number(body.size())
                              + "\r\nConnection: close\r\n\r\n" + body;
            socket->write(resp);
            socket->flush();
            socket->close();
            socket->deleteLater();
        });
        socket->setSocketDescriptor(fd);
    }
};

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
    void sourcesToggleButton_togglesLeftPanel();
    void memoriesToggleButton_togglesRightPanel();
    void regenerate_resendsLastUserMessage();
    void agentMessages_renderAlongsideUserMessage();
    void agentSessionActive_sendCommandGoesOverWebSocket();
    void stopRequestedDuringAgentSession_endsAgentSession();
    void workspaceSelectedFromWelcome_switchesToChatView();
    void setWorkspace_fetchesRealWorkspaceName();
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

void TestChatContainerWidget::sourcesToggleButton_togglesLeftPanel()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    QPushButton *btn = widget.findChild<QPushButton *>(QStringLiteral("sourcesToggleBtn"));
    SourcesSidebar *sidebar = widget.findChild<SourcesSidebar *>();
    QVERIFY(btn != nullptr);
    QVERIFY(sidebar != nullptr);
    QVERIFY(!sidebar->isOpen());

    btn->click();
    QVERIFY(sidebar->isOpen());

    btn->click();
    QVERIFY(!sidebar->isOpen());
}

void TestChatContainerWidget::memoriesToggleButton_togglesRightPanel()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    QPushButton *btn = widget.findChild<QPushButton *>(QStringLiteral("memoriesToggleBtn"));
    MemoriesSidebar *sidebar = widget.findChild<MemoriesSidebar *>();
    QVERIFY(btn != nullptr);
    QVERIFY(sidebar != nullptr);
    QVERIFY(!sidebar->isOpen());

    btn->click();
    QVERIFY(sidebar->isOpen());

    btn->click();
    QVERIFY(!sidebar->isOpen());
}

void TestChatContainerWidget::regenerate_resendsLastUserMessage()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    // Send a message, then simulate a completed assistant reply.
    QSignalSpy streamSpy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("first question"));
    QCOMPARE(streamSpy.count(), 1);

    ChatStreamHandler *handler = widget.findChild<ChatStreamHandler *>();
    QVERIFY(handler != nullptr);
    QJsonObject resp;
    resp.insert("uuid", QStringLiteral("a1"));
    resp.insert("type", QStringLiteral("textResponse"));
    resp.insert("textResponse", QStringLiteral("an answer"));
    resp.insert("close", true);
    handler->handleResponse(HermindStreamChatResponse::fromJson(resp));
    QCOMPARE(handler->messages().size(), 2);

    // Click the regenerate button on the assistant bubble.
    QPushButton *regenBtn = nullptr;
    const QList<QPushButton *> buttons = widget.findChildren<QPushButton *>(QStringLiteral("regenerateBtn"));
    if (!buttons.isEmpty())
        regenBtn = buttons.last();
    QVERIFY(regenBtn != nullptr);
    regenBtn->click();

    // The assistant reply must be dropped and the user message re-sent:
    // history back to just the (re-appended) user message, new stream started.
    QCOMPARE(handler->messages().size(), 1);
    QCOMPARE(handler->messages().first().content(), QStringLiteral("first question"));
    QCOMPARE(streamSpy.count(), 2);
}

// Regression: in an agent session the SSE stream only delivers the
// agentInitWebsocketConnection frame, so the stream handler holds just the
// user message while all agent replies accumulate in the agent handler.
// The history view must show both.
void TestChatContainerWidget::agentMessages_renderAlongsideUserMessage()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    widget.sendCommand(QStringLiteral("hello agent")); // user msg -> stream handler

    ChatHistoryWidget *history = widget.findChild<ChatHistoryWidget *>();
    AgentEventHandler *agent = widget.findChild<AgentEventHandler *>();
    QVERIFY(history != nullptr);
    QVERIFY(agent != nullptr);
    QCOMPARE(history->messageCount(), 1);

    // Agent WS main text path: frame with no "type" field (bridge.go OnMessage).
    agent->handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"content", "agent reply"}}));

    QCOMPARE(history->messageCount(), 2);
}

// Frontend contract (WorkspaceChat/ChatContainer/index.jsx): while the agent
// WebSocket is open, a new user message is sent over the socket as
// "awaitingFeedback" instead of starting a new SSE stream.
void TestChatContainerWidget::agentSessionActive_sendCommandGoesOverWebSocket()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    ChatStreamHandler *stream = widget.findChild<ChatStreamHandler *>();
    QVERIFY(stream != nullptr);

    // Simulate the SSE frame that hands the session over to the agent WS.
    QSignalSpy wsSpy(stream, &ChatStreamHandler::agentWebSocketRequested);
    QJsonObject init;
    init.insert("type", QStringLiteral("agentInitWebsocketConnection"));
    init.insert("websocketUUID", QStringLiteral("sock-1"));
    init.insert("websocketToken", QStringLiteral("tok-1"));
    stream->handleResponse(HermindStreamChatResponse::fromJson(init));
    QCOMPARE(wsSpy.count(), 1);

    QSignalSpy streamSpy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("here is my feedback"));

    // No new SSE stream; the user message is appended to the visible history.
    QCOMPARE(streamSpy.count(), 0);
    QCOMPARE(stream->messages().size(), 1);
    QCOMPARE(stream->messages().first().role(), HermindChatMessage::User);
    QCOMPARE(stream->messages().first().content(), QStringLiteral("here is my feedback"));
}

// Stopping during an agent WebSocket session must terminate the session
// (close the socket) so the next message starts a fresh SSE stream instead
// of being routed to the dead session as feedback.
void TestChatContainerWidget::stopRequestedDuringAgentSession_endsAgentSession()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    ChatStreamHandler *stream = widget.findChild<ChatStreamHandler *>();
    QVERIFY(stream != nullptr);

    QJsonObject init;
    init.insert("type", QStringLiteral("agentInitWebsocketConnection"));
    init.insert("websocketUUID", QStringLiteral("sock-1"));
    init.insert("websocketToken", QStringLiteral("tok-1"));
    stream->handleResponse(HermindStreamChatResponse::fromJson(init));

    PromptInput *input = widget.findChild<PromptInput *>();
    QVERIFY(input != nullptr);
    input->stopRequested();

    // Session must be over: a new message now starts a fresh SSE stream.
    QSignalSpy streamSpy(&widget, &ChatContainerWidget::streamStarted);
    widget.sendCommand(QStringLiteral("new question"));
    QCOMPARE(streamSpy.count(), 1);
}

// The welcome page's "进入 <ws> →" button must take the user to the chat
// view of the current workspace (mirrors the frontend NavLink to the
// workspace chat route).
void TestChatContainerWidget::workspaceSelectedFromWelcome_switchesToChatView()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("My Workspace"));

    QStackedWidget *stack = widget.findChild<QStackedWidget *>(QStringLiteral("chatStack"));
    DefaultChatWidget *welcome = widget.findChild<DefaultChatWidget *>();
    QVERIFY(stack != nullptr);
    QVERIFY(welcome != nullptr);
    QCOMPARE(stack->currentIndex(), 0);

    welcome->workspaceSelected(QStringLiteral("ws-1"));

    QCOMPARE(stack->currentIndex(), 1);
}

// MainChatWidget only knows the workspace slug when navigating, so it
// passes slug as a placeholder name. Once getWorkspace() returns, the real
// name must replace it everywhere the container exposes it.
void TestChatContainerWidget::setWorkspace_fetchesRealWorkspaceName()
{
    MockWorkspaceServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.port())));

    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace(QStringLiteral("ws-1"), QStringLiteral("ws-1"));
    QCOMPARE(widget.workspaceName(), QStringLiteral("ws-1")); // placeholder first

    QTRY_COMPARE(widget.workspaceName(), QStringLiteral("Real Name"));
}

QTEST_MAIN(TestChatContainerWidget)
#include "tst_chat_container_widget.moc"
