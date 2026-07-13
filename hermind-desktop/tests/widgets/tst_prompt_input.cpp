#include <QtTest>
#include <QSignalSpy>
#include <QTextEdit>
#include <QDropEvent>
#include <QDragEnterEvent>
#include <QMimeData>
#include <QUrl>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QFile>
#include <QImage>
#include "prompt_input.h"
#include "hermind_api_client.h"

// Accepts connections and holds them until release() is called, so upload
// processing state can be observed deterministically.
class HoldServer : public QTcpServer
{
    Q_OBJECT
public:
    bool start() { return listen(QHostAddress::LocalHost, 0); }

    int connectionCount() const { return m_sockets.size(); }

    void release(const QByteArray &body)
    {
        const QByteArray response = QByteArrayLiteral("HTTP/1.1 200 OK\r\n"
                                                      "Content-Type: application/json\r\n"
                                                      "Content-Length: ")
                                    + QByteArray::number(body.size())
                                    + QByteArrayLiteral("\r\nConnection: close\r\n\r\n") + body;
        for (QTcpSocket *socket : m_sockets) {
            socket->write(response);
            socket->flush();
            socket->close();
            socket->deleteLater();
        }
        m_sockets.clear();
    }

protected:
    void incomingConnection(qintptr fd) override
    {
        QTcpSocket *socket = new QTcpSocket(this);
        m_sockets.append(socket);
        socket->setSocketDescriptor(fd);
    }

private:
    QVector<QTcpSocket *> m_sockets;
};

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
    void dropWithLocalFileAddsAttachment();
    void dropWithNonUrlMimeIsIgnored();
    void sendCommand_carriesImageDataUrl();
    void sendBlockedWhileAttachmentsProcessing();
    void sendButtonDisabledWhileProcessing();

private:
    static void dropLocalFile(PromptInput &input, const QString &path);
    static void pressEnter(PromptInput &input);
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

void TestPromptInput::dropWithLocalFileAddsAttachment()
{
    PromptInput input;

    QMimeData mime;
    mime.setUrls({ QUrl::fromLocalFile(QStringLiteral("/tmp/test.txt")) });

    // QApplication::notify only delivers Drop events to QDragManager's
    // currentTarget, which is set when a preceding DragEnter is accepted.
    QDragEnterEvent enter(QPoint(10, 10), Qt::CopyAction, &mime,
                          Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &enter);
    QVERIFY(enter.isAccepted());

    QDropEvent drop(QPointF(10, 10), Qt::CopyAction, &mime,
                    Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &drop);

    QVERIFY(input.attachments().contains(QStringLiteral("/tmp/test.txt")));
}

void TestPromptInput::dropWithNonUrlMimeIsIgnored()
{
    PromptInput input;

    QMimeData mime;
    mime.setText(QStringLiteral("not a file"));

    QDragEnterEvent enter(QPoint(10, 10), Qt::CopyAction, &mime,
                          Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &enter);

    QDropEvent drop(QPointF(10, 10), Qt::CopyAction, &mime,
                    Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &drop);

    QVERIFY(input.attachments().isEmpty());
}

void TestPromptInput::dropLocalFile(PromptInput &input, const QString &path)
{
    QMimeData mime;
    mime.setUrls({ QUrl::fromLocalFile(path) });

    QDragEnterEvent enter(QPoint(10, 10), Qt::CopyAction, &mime,
                          Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &enter);
    QVERIFY(enter.isAccepted());

    QDropEvent drop(QPointF(10, 10), Qt::CopyAction, &mime,
                    Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(input.textEdit(), &drop);
}

void TestPromptInput::pressEnter(PromptInput &input)
{
    QKeyEvent *event = new QKeyEvent(QEvent::KeyPress, Qt::Key_Enter, Qt::NoModifier);
    QApplication::postEvent(input.textEdit(), event);
}

void TestPromptInput::sendCommand_carriesImageDataUrl()
{
    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = dir.path() + QStringLiteral("/pic.png");
    QImage img(4, 4, QImage::Format_ARGB32);
    img.fill(Qt::red);
    QVERIFY(img.save(path, "PNG"));

    PromptInput input;
    dropLocalFile(input, path);
    QCOMPARE(input.attachments().size(), 1);

    input.setText(QStringLiteral("look at this"));
    QSignalSpy spy(&input, &PromptInput::sendCommand);
    pressEnter(input);

    QVERIFY(spy.wait(100));
    QCOMPARE(spy.count(), 1);
    const PromptCommand cmd = spy.at(0).at(0).value<PromptCommand>();
    QCOMPARE(cmd.attachments.size(), 1);
    QVERIFY(cmd.attachments.first().startsWith(QStringLiteral("data:image/png;base64,")));
    QVERIFY(input.attachments().isEmpty()); // cleared after send
}

void TestPromptInput::sendBlockedWhileAttachmentsProcessing()
{
    HoldServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.serverPort())));

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = dir.path() + QStringLiteral("/note.txt");
    QFile f(path);
    QVERIFY(f.open(QIODevice::WriteOnly));
    f.write("hi");
    f.close();

    PromptInput input;
    input.setApiClient(&client);
    input.setWorkspaceSlug(QStringLiteral("demo"));
    input.setText(QStringLiteral("with doc"));
    QSignalSpy spy(&input, &PromptInput::sendCommand);

    dropLocalFile(input, path);
    QVERIFY(input.isProcessingAttachments());

    pressEnter(input);
    QTest::qWait(150);
    QCOMPARE(spy.count(), 0); // blocked while upload in flight
    QVERIFY(input.attachments().contains(path)); // nothing cleared

    QTRY_VERIFY_WITH_TIMEOUT(server.connectionCount() >= 1, 5000);
    server.release(QByteArray(R"({"success":true,"document":{"docId":"doc-1"}})"));
    QTRY_VERIFY_WITH_TIMEOUT(!input.isProcessingAttachments(), 5000);

    pressEnter(input);
    QVERIFY(spy.wait(100));
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.at(0).at(0).value<PromptCommand>().text, QStringLiteral("with doc"));
}

void TestPromptInput::sendButtonDisabledWhileProcessing()
{
    HoldServer server;
    QVERIFY(server.start());

    HermindApiClient client;
    client.setBaseUrl(QUrl(QStringLiteral("http://127.0.0.1:%1/api").arg(server.serverPort())));

    QTemporaryDir dir;
    QVERIFY(dir.isValid());
    const QString path = dir.path() + QStringLiteral("/note.txt");
    QFile f(path);
    QVERIFY(f.open(QIODevice::WriteOnly));
    f.write("hi");
    f.close();

    PromptInput input;
    input.setApiClient(&client);
    input.setWorkspaceSlug(QStringLiteral("demo"));
    QVERIFY(input.isSendEnabled());

    dropLocalFile(input, path);
    QVERIFY(!input.isSendEnabled());

    QTRY_VERIFY_WITH_TIMEOUT(server.connectionCount() >= 1, 5000);
    server.release(QByteArray(R"({"success":true,"document":{"docId":"doc-2"}})"));
    QTRY_VERIFY_WITH_TIMEOUT(input.isSendEnabled(), 5000);
}

QTEST_MAIN(TestPromptInput)
#include "tst_prompt_input.moc"
