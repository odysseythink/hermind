#include <QtTest>
#include "api_response.h"
#include "hermind_user.h"
#include "hermind_workspace.h"
#include "hermind_stream_chat_response.h"
#include "hermind_agent_event.h"

class TestModels : public QObject
{
    Q_OBJECT

private slots:
    void apiErrorDefaults();
    void apiResponseSuccess();
    void apiResponseFailure();
    void userFromJson();
    void workspaceFromJson();
    void streamChatResponseFromJson();
    void agentEventFromJson();
};

void TestModels::apiErrorDefaults()
{
    ApiError err;
    QVERIFY(err.isEmpty());
    QCOMPARE(err.httpStatus(), 0);
    QCOMPARE(err.networkError(), QNetworkReply::NoError);
}

void TestModels::apiResponseSuccess()
{
    QJsonObject body;
    body.insert("token", QString("abc123"));

    ApiResponse resp(200, QJsonDocument(body));
    QVERIFY(resp.isSuccess());
    QCOMPARE(resp.statusCode(), 200);
    QCOMPARE(resp.body().object().value("token").toString(), QString("abc123"));
}

void TestModels::apiResponseFailure()
{
    ApiError err("Unauthorized", 401, QNetworkReply::NoError);
    ApiResponse resp(401, QJsonDocument(), err);
    QVERIFY(!resp.isSuccess());
    QCOMPARE(resp.error().message(), QString("Unauthorized"));
    QCOMPARE(resp.error().httpStatus(), 401);
}

void TestModels::userFromJson()
{
    QJsonObject obj;
    obj.insert("id", 7);
    obj.insert("username", QString("alice"));
    obj.insert("role", QString("admin"));
    obj.insert("suspended", 0);
    obj.insert("pfpFilename", QString("alice.png"));

    HermindUser user = HermindUser::fromJson(obj);
    QCOMPARE(user.id(), 7);
    QCOMPARE(user.username(), QString("alice"));
    QCOMPARE(user.role(), QString("admin"));
    QCOMPARE(user.suspended(), 0);
    QVERIFY(user.pfpFilename().has_value());
    QCOMPARE(user.pfpFilename().value(), QString("alice.png"));

    QJsonObject roundTrip = user.toJson();
    QCOMPARE(roundTrip.value("username").toString(), QString("alice"));
}

void TestModels::workspaceFromJson()
{
    QJsonObject obj;
    obj.insert("id", 1);
    obj.insert("name", QString("Default"));
    obj.insert("slug", QString("default"));
    obj.insert("openAiHistory", 20);
    obj.insert("vectorSearchMode", QString("default"));

    HermindWorkspace ws = HermindWorkspace::fromJson(obj);
    QCOMPARE(ws.id(), 1);
    QCOMPARE(ws.name(), QString("Default"));
    QCOMPARE(ws.slug(), QString("default"));
    QCOMPARE(ws.openAiHistory(), 20);
    QCOMPARE(ws.vectorSearchMode(), QString("default"));

    QJsonObject roundTrip = ws.toJson();
    QCOMPARE(roundTrip.value("slug").toString(), QString("default"));
}

void TestModels::streamChatResponseFromJson()
{
    QJsonObject obj;
    obj.insert("uuid", QString("msg-1"));
    obj.insert("type", QString("textResponseChunk"));
    obj.insert("textResponse", QString("hello"));
    obj.insert("close", false);
    obj.insert("chatId", 42);
    obj.insert("animate", true);

    HermindStreamChatResponse resp = HermindStreamChatResponse::fromJson(obj);
    QCOMPARE(resp.uuid(), QString("msg-1"));
    QCOMPARE(resp.type(), QString("textResponseChunk"));
    QVERIFY(resp.textResponse().has_value());
    QCOMPARE(resp.textResponse().value(), QString("hello"));
    QCOMPARE(resp.close(), false);
    QVERIFY(resp.chatId().has_value());
    QCOMPARE(resp.chatId().value(), 42);
    QCOMPARE(resp.animate(), true);

    QJsonObject roundTrip = resp.toJson();
    QCOMPARE(roundTrip.value("type").toString(), QString("textResponseChunk"));
}

void TestModels::agentEventFromJson()
{
    QJsonObject payload;
    payload.insert("query", QString("SELECT 1"));

    QJsonObject obj;
    obj.insert("type", QString("toolApprovalRequest"));
    obj.insert("requestId", QString("req-1"));
    obj.insert("skillName", QString("sql-query"));
    obj.insert("description", QString("run a query"));
    obj.insert("timeoutMs", 120000);
    obj.insert("payload", payload);

    HermindAgentEvent ev = HermindAgentEvent::fromJson(obj);
    QCOMPARE(ev.type(), QString("toolApprovalRequest"));
    QCOMPARE(ev.requestId(), QString("req-1"));
    QCOMPARE(ev.skillName(), QString("sql-query"));
    QCOMPARE(ev.description(), QString("run a query"));
    QCOMPARE(ev.timeoutMs(), 120000);
    QVERIFY(ev.payload().isObject());
    QCOMPARE(ev.payload().toObject().value("query").toString(), QString("SELECT 1"));

    QJsonObject roundTrip = ev.toJson();
    QCOMPARE(roundTrip.value("type").toString(), QString("toolApprovalRequest"));
}

QTEST_MAIN(TestModels)
#include "tst_models.moc"
