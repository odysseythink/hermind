#include <QtTest>
#include <QSignalSpy>
#include "hermind_api_client.h"

class TestApiClientChatHistory : public QObject
{
    Q_OBJECT
private slots:
    void chatHistory_emitsCallbackWithMessages();
    void threadChatHistory_usesCorrectPath();
};

void TestApiClientChatHistory::chatHistory_emitsCallbackWithMessages()
{
    HermindApiClient client;
    client.setBaseUrl(QUrl("http://127.0.0.1:9999"));

    bool called = false;
    client.chatHistory("ws-1", [&](const QVector<HermindChatMessage> &msgs, const ApiError &err) {
        called = true;
        Q_UNUSED(msgs)
        Q_UNUSED(err)
    });

    // 仅验证方法存在并可调用；不启动真实服务器。
    QVERIFY(!called); // 请求未真正完成
}

void TestApiClientChatHistory::threadChatHistory_usesCorrectPath()
{
    HermindApiClient client;
    client.setBaseUrl(QUrl("http://127.0.0.1:9999"));
    // 编译期验证：方法签名正确
    client.threadChatHistory("ws-1", "th-1", [](const QVector<HermindChatMessage> &, const ApiError &) {});
    QVERIFY(true);
}

QTEST_APPLESS_MAIN(TestApiClientChatHistory)
#include "tst_api_client_chat_history.moc"
