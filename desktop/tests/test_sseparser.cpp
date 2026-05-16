#include <QTest>
#include "../src/SSEParser.h"
#include <QSignalSpy>

class TestSSEParser : public QObject
{
    Q_OBJECT
private slots:
    void testSimpleEvent();
    void testMultilineData();
};

void TestSSEParser::testSimpleEvent()
{
    SSEParser parser;
    QSignalSpy spy(&parser, &SSEParser::eventReceived);
    parser.feed("data: hello world\n\n");
    QCOMPARE(spy.count(), 1);
    QList<QVariant> args = spy.takeFirst();
    QCOMPARE(args[0].toString(), QString("message"));
    QCOMPARE(args[1].toString(), QString("hello world"));
}

void TestSSEParser::testMultilineData()
{
    SSEParser parser;
    QSignalSpy spy(&parser, &SSEParser::eventReceived);
    parser.feed("data: line one\ndata: line two\n\n");
    QCOMPARE(spy.count(), 1);
    QList<QVariant> args = spy.takeFirst();
    QCOMPARE(args[1].toString(), QString("line one\nline two"));
}

QTEST_MAIN(TestSSEParser)
#include "test_sseparser.moc"
