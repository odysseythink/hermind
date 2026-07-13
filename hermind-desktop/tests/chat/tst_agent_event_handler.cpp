#include <QtTest>
#include <QSignalSpy>
#include "agent_event_handler.h"

class TestAgentEventHandler : public QObject
{
    Q_OBJECT
private slots:
    // Generic no-type frames (backend bridge.go OnMessage: the real agent text path)
    void genericMessage_appendsAssistantMessage();
    void genericMessage_emptyContent_ignored();

    // reportStreamEvent with object content (frontend agent.js contract)
    void reportStreamEvent_stringContent_ignored();
    void reportStreamEvent_textResponseChunk_createsAndAppends();
    void reportStreamEvent_firstChunkWhitespaceOnly_ignored();
    void reportStreamEvent_fullTextResponse_createsMessage();
    void reportStreamEvent_removeStatusResponse_removesMessage();
    void reportStreamEvent_toolCallInvocation_replacesContent();
    void reportStreamEvent_citations_attachesToExistingMessage();
    void reportStreamEvent_citations_bufferedUntilMessageArrives();

    // Pre-existing event types
    void statusResponse_emitsStatusSignal();
    void fileDownloadCard_emitsDownloadCardSignal();
    void clarificationRequest_emitsRequestSignal();
};

static QJsonObject streamEventObj(const QString &subType, const QString &uuid,
                                  const QString &content)
{
    QJsonObject inner;
    inner.insert("type", subType);
    if (!uuid.isEmpty())
        inner.insert("uuid", uuid);
    if (!content.isNull())
        inner.insert("content", content);
    QJsonObject obj;
    obj.insert("type", QStringLiteral("reportStreamEvent"));
    obj.insert("content", inner);
    return obj;
}

void TestAgentEventHandler::genericMessage_appendsAssistantMessage()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::messagesChanged);

    QJsonObject obj;
    obj.insert("content", QStringLiteral("Agent says hi"));
    obj.insert("uuid", QStringLiteral("u-1"));
    obj.insert("from", QStringLiteral("agent"));
    obj.insert("to", QStringLiteral("user"));
    obj.insert("state", QStringLiteral("complete"));
    handler.handleEvent(HermindAgentEvent::fromJson(obj));

    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QStringLiteral("Agent says hi"));
    QCOMPARE(handler.messages().first().uuid(), QStringLiteral("u-1"));
    QVERIFY(handler.messages().first().role() == HermindChatMessage::Assistant);
    QVERIFY(handler.messages().first().isClosed());
    QCOMPARE(spy.count(), 1);
}

void TestAgentEventHandler::genericMessage_emptyContent_ignored()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::messagesChanged);

    QJsonObject obj;
    obj.insert("from", QStringLiteral("agent"));
    handler.handleEvent(HermindAgentEvent::fromJson(obj));

    QCOMPARE(handler.messages().size(), 0);
    QCOMPARE(spy.count(), 0);
}

void TestAgentEventHandler::reportStreamEvent_stringContent_ignored()
{
    // Regression: previously string content created an empty assistant message
    // with a constant qHash-based uuid.
    AgentEventHandler handler;
    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "reportStreamEvent"}, {"content", "Hi"}}));
    QCOMPARE(handler.messages().size(), 0);
}

void TestAgentEventHandler::reportStreamEvent_textResponseChunk_createsAndAppends()
{
    AgentEventHandler handler;

    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("textResponseChunk"), QStringLiteral("u-9"), QStringLiteral("Hello"))));
    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QStringLiteral("Hello"));
    QCOMPARE(handler.messages().first().uuid(), QStringLiteral("u-9"));

    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("textResponseChunk"), QStringLiteral("u-9"), QStringLiteral(" world"))));
    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QStringLiteral("Hello world"));
}

void TestAgentEventHandler::reportStreamEvent_firstChunkWhitespaceOnly_ignored()
{
    AgentEventHandler handler;
    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("textResponseChunk"), QStringLiteral("u-9"), QStringLiteral("\n"))));
    QCOMPARE(handler.messages().size(), 0);
}

void TestAgentEventHandler::reportStreamEvent_fullTextResponse_createsMessage()
{
    AgentEventHandler handler;
    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("fullTextResponse"), QStringLiteral("u-2"), QStringLiteral("Full answer"))));
    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QStringLiteral("Full answer"));
    QVERIFY(handler.messages().first().isClosed());
}

void TestAgentEventHandler::reportStreamEvent_removeStatusResponse_removesMessage()
{
    AgentEventHandler handler;
    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("fullTextResponse"), QStringLiteral("u-3"), QStringLiteral("temp"))));
    QCOMPARE(handler.messages().size(), 1);

    QJsonObject inner;
    inner.insert("type", QStringLiteral("removeStatusResponse"));
    inner.insert("uuid", QStringLiteral("u-3"));
    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "reportStreamEvent"}, {"content", inner}}));
    QCOMPARE(handler.messages().size(), 0);
}

void TestAgentEventHandler::reportStreamEvent_toolCallInvocation_replacesContent()
{
    AgentEventHandler handler;
    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("toolCallInvocation"), QStringLiteral("u-4"), QStringLiteral("search(\"a\""))));
    handler.handleEvent(HermindAgentEvent::fromJson(
        streamEventObj(QStringLiteral("toolCallInvocation"), QStringLiteral("u-4"), QStringLiteral("search(\"abc\")"))));
    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QStringLiteral("search(\"abc\")"));
}

void TestAgentEventHandler::reportStreamEvent_citations_attachesToExistingMessage()
{
    AgentEventHandler handler;
    QSignalSpy citeSpy(&handler, &AgentEventHandler::citationsReceived);

    QJsonObject msg;
    msg.insert("content", QStringLiteral("answer"));
    msg.insert("uuid", QStringLiteral("u-5"));
    handler.handleEvent(HermindAgentEvent::fromJson(msg));

    QJsonObject inner;
    inner.insert("type", QStringLiteral("citations"));
    inner.insert("uuid", QStringLiteral("u-5"));
    inner.insert("citations", QJsonArray{QJsonObject{{"title", "doc1"}}});
    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "reportStreamEvent"}, {"content", inner}}));

    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().sources().size(), 1);
    QCOMPARE(citeSpy.count(), 1);
    QCOMPARE(citeSpy.first().at(0).toString(), QStringLiteral("u-5"));
}

void TestAgentEventHandler::reportStreamEvent_citations_bufferedUntilMessageArrives()
{
    AgentEventHandler handler;

    QJsonObject inner;
    inner.insert("type", QStringLiteral("citations"));
    inner.insert("uuid", QStringLiteral("u-6"));
    inner.insert("citations", QJsonArray{QJsonObject{{"title", "doc2"}}});
    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "reportStreamEvent"}, {"content", inner}}));
    QCOMPARE(handler.messages().size(), 0);

    QJsonObject msg;
    msg.insert("content", QStringLiteral("late answer"));
    msg.insert("uuid", QStringLiteral("u-6"));
    handler.handleEvent(HermindAgentEvent::fromJson(msg));

    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().sources().size(), 1);
}

void TestAgentEventHandler::statusResponse_emitsStatusSignal()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::statusReceived);

    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "statusResponse"}, {"content", "Thinking..."}}));

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.first().first().toString(), QStringLiteral("Thinking..."));
}

void TestAgentEventHandler::fileDownloadCard_emitsDownloadCardSignal()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::downloadCardReceived);

    handler.handleEvent(HermindAgentEvent::fromJson(QJsonObject{{"type", "fileDownloadCard"}}));

    QCOMPARE(spy.count(), 1);
}

void TestAgentEventHandler::clarificationRequest_emitsRequestSignal()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::clarificationRequested);

    handler.handleEvent(HermindAgentEvent::fromJson(
        QJsonObject{{"type", "clarificationRequest"}, {"question", "Which file?"}}));

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.first().first().toString(), QStringLiteral("Which file?"));
}

QTEST_APPLESS_MAIN(TestAgentEventHandler)
#include "tst_agent_event_handler.moc"
