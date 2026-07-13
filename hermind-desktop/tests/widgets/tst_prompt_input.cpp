#include <QtTest>
#include <QSignalSpy>
#include <QTextEdit>
#include "prompt_input.h"

class TestPromptInput : public QObject
{
    Q_OBJECT
private slots:
    void initTestCase();
    void returnsEmptyTextWhenNoInput();
    void getTextReturnsTrimmedContent();
    void setTextAndGetBack();
    void clearEmptiesContent();
    void enterEmitsSendCommand();
    void shiftEnterInsertsNewlineWithoutSending();
    void emptyTextDoesNotEmitSendOnEnter();
};

void TestPromptInput::initTestCase()
{
    qRegisterMetaType<PromptCommand>("PromptCommand");
}

void TestPromptInput::returnsEmptyTextWhenNoInput()
{
    PromptInput input;
    QCOMPARE(input.text(), QString());
}

void TestPromptInput::getTextReturnsTrimmedContent()
{
    PromptInput input;
    input.setText(QStringLiteral("  hello world  "));
    QCOMPARE(input.text(), QStringLiteral("hello world"));
}

void TestPromptInput::setTextAndGetBack()
{
    PromptInput input;
    input.setText(QStringLiteral("test message"));
    QCOMPARE(input.text(), QStringLiteral("test message"));
}

void TestPromptInput::clearEmptiesContent()
{
    PromptInput input;
    input.setText(QStringLiteral("something"));
    input.clear();
    QCOMPARE(input.text(), QString());
}

void TestPromptInput::enterEmitsSendCommand()
{
    PromptInput input;
    input.setText(QStringLiteral("hello"));

    QSignalSpy spy(&input, &PromptInput::sendCommand);

    QKeyEvent *event = new QKeyEvent(QEvent::KeyPress, Qt::Key_Enter, Qt::NoModifier);
    QApplication::postEvent(input.textEdit(), event);

    QVERIFY(spy.wait(100));
    QCOMPARE(spy.count(), 1);
    const PromptCommand cmd = spy.at(0).at(0).value<PromptCommand>();
    QCOMPARE(cmd.text, QStringLiteral("hello"));
    QCOMPARE(cmd.writeMode, QStringLiteral("replace"));
}

void TestPromptInput::shiftEnterInsertsNewlineWithoutSending()
{
    PromptInput input;
    input.setText(QStringLiteral("line1"));

    QSignalSpy spy(&input, &PromptInput::sendCommand);

    QKeyEvent *event = new QKeyEvent(QEvent::KeyPress, Qt::Key_Return,
                                      Qt::ShiftModifier);
    QApplication::postEvent(input.textEdit(), event);

    QTest::qWait(50);
    QVERIFY(input.text().contains(QStringLiteral("line1")));
    QCOMPARE(spy.count(), 0);
}

void TestPromptInput::emptyTextDoesNotEmitSendOnEnter()
{
    PromptInput input;
    input.clear();

    QSignalSpy spy(&input, &PromptInput::sendCommand);

    QKeyEvent *event = new QKeyEvent(QEvent::KeyPress, Qt::Key_Enter, Qt::NoModifier);
    QApplication::postEvent(input.textEdit(), event);

    QTest::qWait(50);
    QCOMPARE(spy.count(), 0);
}

QTEST_MAIN(TestPromptInput)
#include "tst_prompt_input.moc"
