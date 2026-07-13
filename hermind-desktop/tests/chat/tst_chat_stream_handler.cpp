#include <QtTest>
#include <QSignalSpy>
#include "chat_stream_handler.h"
#include "hermind_stream_chat_response.h"

class TestChatStreamHandler : public QObject
{
    Q_OBJECT
private slots:
    void textResponseChunk_appendsToAssistantMessage();
    void finalizeResponseStream_marksMessageClosed();
    void stopGenerationAction_clearsPendingStream();
};

static HermindStreamChatResponse makeResponse(const QString &uuid,
                                              const QString &type,
                                              const QString &text = QString(),
                                              bool close = false)
{
    QJsonObject obj;
    obj.insert("uuid", uuid);
    obj.insert("type", type);
    if (!text.isEmpty())
        obj.insert("textResponse", text);
    obj.insert("close", close);
    return HermindStreamChatResponse::fromJson(obj);
}

void TestChatStreamHandler::textResponseChunk_appendsToAssistantMessage()
{
    ChatStreamHandler handler;
    QSignalSpy spy(&handler, &ChatStreamHandler::messagesChanged);

    handler.handleResponse(makeResponse("u1", "textResponseChunk", "Hel"));
    handler.handleResponse(makeResponse("u1", "textResponseChunk", "lo"));

    const QVector<HermindChatMessage> msgs = handler.messages();
    QCOMPARE(msgs.size(), 1);
    QCOMPARE(msgs.first().uuid(), QString("u1"));
    QCOMPARE(msgs.first().content(), QString("Hello"));
    QVERIFY(msgs.first().role() == HermindChatMessage::Assistant);
    QVERIFY(spy.count() >= 2);
}

void TestChatStreamHandler::finalizeResponseStream_marksMessageClosed()
{
    ChatStreamHandler handler;
    handler.handleResponse(makeResponse("u1", "textResponseChunk", "Hel"));
    handler.handleResponse(makeResponse("u1", "finalizeResponseStream", QString(), true));

    QVERIFY(handler.messages().first().isClosed());
}

void TestChatStreamHandler::stopGenerationAction_clearsPendingStream()
{
    ChatStreamHandler handler;
    handler.handleResponse(makeResponse("u1", "textResponseChunk", "Hel"));
    handler.handleResponse(makeResponse("", "stopGeneration"));

    QVERIFY(handler.messages().first().isClosed());
}

QTEST_APPLESS_MAIN(TestChatStreamHandler)
#include "tst_chat_stream_handler.moc"
