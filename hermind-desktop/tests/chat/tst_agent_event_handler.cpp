#include <QtTest>
#include <QSignalSpy>
#include "agent_event_handler.h"

class TestAgentEventHandler : public QObject
{
    Q_OBJECT
private slots:
    void reportStreamEvent_appendsAssistantText();
    void fileDownloadCard_emitsDownloadCardSignal();
    void clarificationRequest_emitsRequestSignal();
};

static QJsonObject eventObj(const QString &type, const QString &content = QString())
{
    QJsonObject obj;
    obj.insert("type", type);
    if (!content.isEmpty())
        obj.insert("content", content);
    return obj;
}

void TestAgentEventHandler::reportStreamEvent_appendsAssistantText()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::messagesChanged);

    handler.handleEvent(HermindAgentEvent::fromJson(eventObj("reportStreamEvent", "Hi")));

    QCOMPARE(handler.messages().size(), 1);
    QCOMPARE(handler.messages().first().content(), QString("Hi"));
    QVERIFY(handler.messages().first().role() == HermindChatMessage::Assistant);
    QVERIFY(spy.count() == 1);
}

void TestAgentEventHandler::fileDownloadCard_emitsDownloadCardSignal()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::downloadCardReceived);

    handler.handleEvent(HermindAgentEvent::fromJson(eventObj("fileDownloadCard")));

    QCOMPARE(spy.count(), 1);
}

void TestAgentEventHandler::clarificationRequest_emitsRequestSignal()
{
    AgentEventHandler handler;
    QSignalSpy spy(&handler, &AgentEventHandler::clarificationRequested);

    QJsonObject obj = eventObj("clarificationRequest");
    obj.insert("question", "Which file?");
    handler.handleEvent(HermindAgentEvent::fromJson(obj));

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.first().first().toString(), QString("Which file?"));
}

QTEST_APPLESS_MAIN(TestAgentEventHandler)
#include "tst_agent_event_handler.moc"
