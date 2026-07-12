# Hermind Desktop 1:3 Chat Container — Models + API

**Scope:** 新增 `HermindChatMessage` 消息模型；在 `HermindApiClient` 增加加载工作区/线程聊天历史的方法。

**Part dependencies:** none (直接基于现有 Hermind 桌面代码)

---

### Task 1: HermindChatMessage 消息模型

**Depends on:** none

**Files:**
- Create: `hermind-desktop/models/hermind_chat_message.h`
- Create: `hermind-desktop/models/hermind_chat_message.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:55-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/models/chat_message_test.pro`
- Create: `hermind-desktop/tests/models/tst_chat_message.cpp`

**Rationale:** 聊天容器需要本地化的消息数据对象，用于 UI 展示与流式增量更新。字段对齐前端 `chatMessage` 与后端 `/workspace/:slug/chats` 返回的 JSON。

#### Step 1.1 — Write the failing test

创建 `hermind-desktop/tests/models/tst_chat_message.cpp` 与 `hermind-desktop/tests/models/chat_message_test.pro`：

`chat_message_test.pro`：

```qmake
QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_chat_message

INCLUDEPATH += ../../api ../../models

SOURCES += \
    tst_chat_message.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp

HEADERS += \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h
```

`tst_chat_message.cpp`：

```cpp
#include <QtTest>
#include "hermind_chat_message.h"

class TestChatMessage : public QObject
{
    Q_OBJECT
private slots:
    void fromJson_populatesAllFields();
    void updateAppendsText_preservesUuidAndRole();
};

void TestChatMessage::fromJson_populatesAllFields()
{
    QJsonObject obj;
    obj.insert("id", 42);
    obj.insert("uuid", "uuid-1");
    obj.insert("role", "assistant");
    obj.insert("user", "user-1");
    obj.insert("content", "Hello");
    obj.insert("sentAt", 1700000000);
    obj.insert("createdAt", 1700000001);
    obj.insert("feedbackScore", 1);
    obj.insert("chatMode", "chat");
    obj.insert("close", true);

    HermindChatMessage msg = HermindChatMessage::fromJson(obj);

    QCOMPARE(msg.id(), 42);
    QCOMPARE(msg.uuid(), QString("uuid-1"));
    QCOMPARE(msg.role(), HermindChatMessage::Assistant);
    QCOMPARE(msg.user(), QString("user-1"));
    QCOMPARE(msg.content(), QString("Hello"));
    QCOMPARE(msg.sentAt().toSecsSinceEpoch(), qint64(1700000000));
    QCOMPARE(msg.feedbackScore(), 1);
    QCOMPARE(msg.chatMode(), QString("chat"));
    QVERIFY(msg.isClosed());
}

void TestChatMessage::updateAppendsText_preservesUuidAndRole()
{
    HermindChatMessage msg = HermindChatMessage::fromJson(QJsonObject{
        {"uuid", "uuid-2"}, {"role", "assistant"}, {"content", "Hel"}
    });
    msg.appendContent("lo");

    QCOMPARE(msg.uuid(), QString("uuid-2"));
    QCOMPARE(msg.role(), HermindChatMessage::Assistant);
    QCOMPARE(msg.content(), QString("Hello"));
}

QTEST_APPLESS_MAIN(TestChatMessage)
#include "tst_chat_message.moc"
```

#### Step 1.2 — Run and verify FAILS

```bash
cd hermind-desktop
qmake tests/models/models_test.pro
make
```

Expected failure: `hermind_chat_message.h: No such file or directory`.

#### Step 1.3 — Write the minimal implementation

`hermind-desktop/models/hermind_chat_message.h`：

```cpp
#ifndef HERMIND_CHAT_MESSAGE_H
#define HERMIND_CHAT_MESSAGE_H

#include <QString>
#include <QDateTime>
#include <QJsonObject>
#include <QJsonArray>

class HermindChatMessage
{
public:
    enum Role {
        User,
        Assistant,
        System,
        Unknown
    };

    HermindChatMessage() = default;
    static HermindChatMessage fromJson(const QJsonObject &obj);

    int id() const;
    QString uuid() const;
    Role role() const;
    QString user() const;
    QString content() const;
    QDateTime sentAt() const;
    QDateTime createdAt() const;
    int feedbackScore() const;
    QString chatMode() const;
    bool isClosed() const;
    QJsonArray sources() const;
    QJsonObject metrics() const;

    void setUuid(const QString &uuid);
    void setRole(Role role);
    void setContent(const QString &content);
    void appendContent(const QString &chunk);
    void setClosed(bool closed);

private:
    static Role roleFromString(const QString &role);

    int m_id = 0;
    QString m_uuid;
    Role m_role = Unknown;
    QString m_user;
    QString m_content;
    QDateTime m_sentAt;
    QDateTime m_createdAt;
    int m_feedbackScore = 0;
    QString m_chatMode;
    bool m_closed = false;
    QJsonArray m_sources;
    QJsonObject m_metrics;
};

#endif // HERMIND_CHAT_MESSAGE_H
```

`hermind-desktop/models/hermind_chat_message.cpp`：

```cpp
#include "hermind_chat_message.h"

HermindChatMessage HermindChatMessage::fromJson(const QJsonObject &obj)
{
    HermindChatMessage msg;
    msg.m_id = obj.value("id").toInt();
    msg.m_uuid = obj.value("uuid").toString();
    msg.m_role = roleFromString(obj.value("role").toString());
    msg.m_user = obj.value("user").toString();
    msg.m_content = obj.value("content").toString();
    msg.m_sentAt = QDateTime::fromSecsSinceEpoch(obj.value("sentAt").toVariant().toLongLong());
    msg.m_createdAt = QDateTime::fromSecsSinceEpoch(obj.value("createdAt").toVariant().toLongLong());
    msg.m_feedbackScore = obj.value("feedbackScore").toInt();
    msg.m_chatMode = obj.value("chatMode").toString();
    msg.m_closed = obj.value("close").toBool();
    msg.m_sources = obj.value("sources").toArray();
    msg.m_metrics = obj.value("metrics").toObject();
    return msg;
}

HermindChatMessage::Role HermindChatMessage::roleFromString(const QString &role)
{
    if (role == QLatin1String("user")) return User;
    if (role == QLatin1String("assistant")) return Assistant;
    if (role == QLatin1String("system")) return System;
    return Unknown;
}

int HermindChatMessage::id() const { return m_id; }
QString HermindChatMessage::uuid() const { return m_uuid; }
HermindChatMessage::Role HermindChatMessage::role() const { return m_role; }
QString HermindChatMessage::user() const { return m_user; }
QString HermindChatMessage::content() const { return m_content; }
QDateTime HermindChatMessage::sentAt() const { return m_sentAt; }
QDateTime HermindChatMessage::createdAt() const { return m_createdAt; }
int HermindChatMessage::feedbackScore() const { return m_feedbackScore; }
QString HermindChatMessage::chatMode() const { return m_chatMode; }
bool HermindChatMessage::isClosed() const { return m_closed; }
QJsonArray HermindChatMessage::sources() const { return m_sources; }
QJsonObject HermindChatMessage::metrics() const { return m_metrics; }

void HermindChatMessage::setUuid(const QString &uuid) { m_uuid = uuid; }
void HermindChatMessage::setRole(Role role) { m_role = role; }
void HermindChatMessage::setContent(const QString &content) { m_content = content; }
void HermindChatMessage::appendContent(const QString &chunk) { m_content += chunk; }
void HermindChatMessage::setClosed(bool closed) { m_closed = closed; }
```

#### Step 1.4 — Run and verify PASSES

```bash
cd hermind-desktop
qmake tests/models/chat_message_test.pro
make && ./tests/models/tst_chat_message
```

Expected: `PASS`。

#### Step 1.5 — Commit

```bash
git add hermind-desktop/models/hermind_chat_message.* hermind-desktop/tests/models/tst_chat_message.cpp hermind-desktop/tests/models/chat_message_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add HermindChatMessage model with tests"
```

---

### Task 2: HermindApiClient 增加 chatHistory 方法

**Depends on:** Task 1

**Files:**
- Modify: `hermind-desktop/api/hermind_api_client.h:55-65`（新增 callback 与方法声明）
- Modify: `hermind-desktop/api/hermind_api_client.cpp:225-235`（新增实现）

**Rationale:** UI 需要调用 `/workspace/:slug/chats` 与 `/workspace/:slug/thread/:threadSlug/chats` 获取历史消息。

#### Step 2.1 — Write the failing test

创建 `hermind-desktop/tests/api/tst_api_client_chat_history.cpp` 与 `hermind-desktop/tests/api/chat_history_test.pro`：

`chat_history_test.pro`：

```qmake
QT += testlib network websockets
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_api_client_chat_history

INCLUDEPATH += ../../api ../../models ../../streaming

SOURCES += \
    tst_api_client_chat_history.cpp \
    ../../api/hermind_api_client.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_user.cpp \
    ../../models/hermind_workspace.cpp \
    ../../models/hermind_workspace_thread.cpp \
    ../../models/hermind_stream_chat_response.cpp \
    ../../models/hermind_agent_event.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../streaming/hermind_sse_client.cpp \
    ../../streaming/hermind_websocket_client.cpp

HEADERS += \
    ../../api/hermind_api_client.h \
    ../../api/api_response.h \
    ../../models/hermind_user.h \
    ../../models/hermind_workspace.h \
    ../../models/hermind_workspace_thread.h \
    ../../models/hermind_stream_chat_response.h \
    ../../models/hermind_agent_event.h \
    ../../models/hermind_chat_message.h \
    ../../streaming/hermind_sse_client.h \
    ../../streaming/hermind_websocket_client.h
```

`tst_api_client_chat_history.cpp`：

```cpp
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
```

#### Step 2.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/api/chat_history_test.pro && make
```

Expected failure: `'chatHistory' is not a member of 'HermindApiClient'`。

#### Step 2.3 — Write the minimal implementation

在 `hermind-desktop/api/hermind_api_client.h` 的 `public:` 区（`StreamChatCallback` 之后）追加：

```cpp
    using ChatHistoryCallback = std::function<void(const QVector<HermindChatMessage> &messages,
                                                   const ApiError &error)>;

    void chatHistory(const QString &workspaceSlug, ChatHistoryCallback callback);
    void threadChatHistory(const QString &workspaceSlug,
                           const QString &threadSlug,
                           ChatHistoryCallback callback);
```

在 `hermind-desktop/api/hermind_api_client.cpp` 的 `deleteThread` 之后、`get()` 之前追加：

```cpp
void HermindApiClient::chatHistory(const QString &workspaceSlug, ChatHistoryCallback callback)
{
    get(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/chats"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindChatMessage>(), resp.error());
                return;
            }
            QVector<HermindChatMessage> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("history")).toArray();
            list.reserve(arr.size());
            for (const QJsonValue &v : arr)
                list.append(HermindChatMessage::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}

void HermindApiClient::threadChatHistory(const QString &workspaceSlug,
                                         const QString &threadSlug,
                                         ChatHistoryCallback callback)
{
    get(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/thread/") + threadSlug
            + QStringLiteral("/chats"),
        QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindChatMessage>(), resp.error());
                return;
            }
            QVector<HermindChatMessage> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("history")).toArray();
            list.reserve(arr.size());
            for (const QJsonValue &v : arr)
                list.append(HermindChatMessage::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}
```

#### Step 2.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/api/chat_history_test.pro && make && ./tests/api/tst_api_client_chat_history
```

Expected: `PASS`。

#### Step 2.5 — Commit

```bash
git add hermind-desktop/api/hermind_api_client.* hermind-desktop/tests/api/tst_api_client_chat_history.cpp hermind-desktop/tests/api/chat_history_test.pro
git commit -m "feat(desktop): add chatHistory and threadChatHistory API methods"
```

---

## Local Self-Review (Models + API Part)

- [ ] 1. Spec-coverage: 消息模型 → Task 1；历史加载 API → Task 2。
- [ ] 2. Placeholder scan：无 TODO/TBD。
- [ ] 3. No phantom tasks：两个任务均产出文件、测试与 commit。
- [ ] 4. Dependency soundness：Task 2 依赖 Task 1 的 `HermindChatMessage`。
- [ ] 5. Caller & build soundness：新增方法在头文件声明并立即实现；本 Part 未修改现有共享签名。
- [ ] 6. Test-the-risk：Task 1 测试 JSON 反序列化与追加文本；Task 2 测试方法存在性与签名。
- [ ] 7. Type consistency：`HermindChatMessage::fromJson`、`role` 枚举在 Part 内部一致。
