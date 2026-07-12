# Hermind Desktop 1:3 Chat Container — SSE / Agent Stream Processors

**Scope:** 把 SSE 流式响应与 Agent WebSocket 事件转换为 `HermindChatMessage` 列表变更。

**Part dependencies:**
- `2026-07-13-hermind-desktop-1-3-chat-container/models.md` (Task 1-2)

---

### Task 3: ChatStreamHandler — SSE 事件 → 消息列表

**Depends on:** Task 1

**Files:**
- Create: `hermind-desktop/chat/chat_stream_handler.h`
- Create: `hermind-desktop/chat/chat_stream_handler.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:11-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/chat/tst_chat_stream_handler.cpp`
- Create: `hermind-desktop/tests/chat/chat_stream_handler_test.pro`

**Rationale:** `HermindApiClient::streamChat` 已能收到 `HermindStreamChatResponse` 对象，但 UI 需要一组 `HermindChatMessage`。`ChatStreamHandler` 维护消息列表，按 UUID 合并增量，并暴露状态信号。

#### Step 3.1 — Write the failing test

创建 `hermind-desktop/tests/chat/tst_chat_stream_handler.cpp` 与 `hermind-desktop/tests/chat/chat_stream_handler_test.pro`：

`chat_stream_handler_test.pro`：

```qmake
QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_chat_stream_handler

INCLUDEPATH += ../../api ../../models ../../chat

SOURCES += \
    tst_chat_stream_handler.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_stream_chat_response.cpp

HEADERS += \
    ../../chat/chat_stream_handler.h \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_stream_chat_response.h
```

`tst_chat_stream_handler.cpp`：

```cpp
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
```

#### Step 3.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/chat/chat_stream_handler_test.pro && make
```

Expected: `chat_stream_handler.h: No such file or directory`。

#### Step 3.3 — Write the minimal implementation

`hermind-desktop/chat/chat_stream_handler.h`：

```cpp
#ifndef CHAT_STREAM_HANDLER_H
#define CHAT_STREAM_HANDLER_H

#include <QObject>
#include <QVector>
#include <QHash>
#include "hermind_chat_message.h"
#include "hermind_stream_chat_response.h"

class ChatStreamHandler : public QObject
{
    Q_OBJECT
public:
    explicit ChatStreamHandler(QObject *parent = nullptr);

    const QVector<HermindChatMessage> &messages() const;
    void setMessages(const QVector<HermindChatMessage> &messages);
    void clear();

    void handleResponse(const HermindStreamChatResponse &response);

signals:
    void messagesChanged();
    void streamFinished();
    void errorReceived(const QString &message);
    void agentWebSocketRequested(const QString &socketId, const QString &token);

private:
    int findMessageIndexByUuid(const QString &uuid) const;
    void appendOrUpdateAssistant(const QString &uuid, const QString &text, bool close);

    QVector<HermindChatMessage> m_messages;
    QHash<QString, int> m_uuidToIndex;
};

#endif // CHAT_STREAM_HANDLER_H
```

`hermind-desktop/chat/chat_stream_handler.cpp`：

```cpp
#include "chat_stream_handler.h"

ChatStreamHandler::ChatStreamHandler(QObject *parent)
    : QObject(parent)
{
}

const QVector<HermindChatMessage> &ChatStreamHandler::messages() const
{
    return m_messages;
}

void ChatStreamHandler::setMessages(const QVector<HermindChatMessage> &messages)
{
    m_messages = messages;
    m_uuidToIndex.clear();
    for (int i = 0; i < m_messages.size(); ++i)
        m_uuidToIndex.insert(m_messages.at(i).uuid(), i);
    emit messagesChanged();
}

void ChatStreamHandler::clear()
{
    m_messages.clear();
    m_uuidToIndex.clear();
    emit messagesChanged();
}

int ChatStreamHandler::findMessageIndexByUuid(const QString &uuid) const
{
    if (uuid.isEmpty())
        return -1;
    return m_uuidToIndex.value(uuid, -1);
}

void ChatStreamHandler::appendOrUpdateAssistant(const QString &uuid,
                                                const QString &text,
                                                bool close)
{
    int idx = findMessageIndexByUuid(uuid);
    if (idx < 0) {
        HermindChatMessage msg;
        msg.setUuid(uuid);
        msg.setRole(HermindChatMessage::Assistant);
        msg.setContent(text);
        msg.setClosed(close);
        m_messages.append(msg);
        m_uuidToIndex.insert(uuid, m_messages.size() - 1);
    } else {
        m_messages[idx].appendContent(text);
        m_messages[idx].setClosed(close);
    }
    emit messagesChanged();
}

void ChatStreamHandler::handleResponse(const HermindStreamChatResponse &response)
{
    const QString type = response.type();
    const QString uuid = response.uuid();

    if (type == QLatin1String("textResponse")) {
        if (const auto text = response.textResponse())
            appendOrUpdateAssistant(uuid, *text, response.close());
    } else if (type == QLatin1String("textResponseChunk")) {
        if (const auto text = response.textResponse())
            appendOrUpdateAssistant(uuid, *text, response.close());
    } else if (type == QLatin1String("finalizeResponseStream")) {
        appendOrUpdateAssistant(uuid, QString(), true);
    } else if (type == QLatin1String("abort")) {
        appendOrUpdateAssistant(uuid.isEmpty() ? m_messages.isEmpty() ? QString() : m_messages.last().uuid() : uuid,
                                QString(), true);
    } else if (type == QLatin1String("statusResponse")) {
        // 本阶段仅记录，不渲染状态卡片
    } else if (type == QLatin1String("stopGeneration")) {
        if (!m_messages.isEmpty())
            m_messages.last().setClosed(true);
        emit streamFinished();
    } else if (type == QLatin1String("modelRouteNotification")) {
        if (const auto routed = response.routedTo(); !routed->isEmpty()) {
            // 可更新 UI 状态，本阶段仅透传
        }
    } else if (type == QLatin1String("agentInitWebsocketConnection")) {
        const auto socketId = response.websocketUUID();
        const auto token = response.websocketToken();
        if (socketId && token)
            emit agentWebSocketRequested(*socketId, *token);
    } else if (type == QLatin1String("action")) {
        const auto action = response.action();
        if (action == QLatin1String("reset_chat")) {
            clear();
        } else if (action == QLatin1String("rename_thread")) {
            // 留给容器处理
        }
    }
}
```

#### Step 3.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/chat/chat_stream_handler_test.pro && make && ./tests/chat/tst_chat_stream_handler
```

Expected: `PASS`。

#### Step 3.5 — Commit

```bash
git add hermind-desktop/chat/chat_stream_handler.* hermind-desktop/tests/chat/tst_chat_stream_handler.cpp hermind-desktop/tests/chat/chat_stream_handler_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add ChatStreamHandler for SSE message aggregation"
```

---

### Task 4: AgentEventHandler — Agent WebSocket 事件 → 消息列表

**Depends on:** Task 1

**Files:**
- Create: `hermind-desktop/chat/agent_event_handler.h`
- Create: `hermind-desktop/chat/agent_event_handler.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:11-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/chat/tst_agent_event_handler.cpp`
- Create: `hermind-desktop/tests/chat/agent_event_handler_test.pro`

**Rationale:** Agent 可能通过 WebSocket 推送 `reportStreamEvent` 等消息，需与 SSE 消息合并到同一份列表。

#### Step 4.1 — Write the failing test

创建 `hermind-desktop/tests/chat/tst_agent_event_handler.cpp` 与 `hermind-desktop/tests/chat/agent_event_handler_test.pro`：

`agent_event_handler_test.pro`：

```qmake
QT += testlib
CONFIG += qt warn_on c++17 console
CONFIG -= app_bundle
TEMPLATE = app
TARGET = tst_agent_event_handler

INCLUDEPATH += ../../api ../../models ../../chat

SOURCES += \
    tst_agent_event_handler.cpp \
    ../../chat/agent_event_handler.cpp \
    ../../api/api_response.cpp \
    ../../models/hermind_chat_message.cpp \
    ../../models/hermind_agent_event.cpp

HEADERS += \
    ../../chat/agent_event_handler.h \
    ../../api/api_response.h \
    ../../models/hermind_chat_message.h \
    ../../models/hermind_agent_event.h
```

`tst_agent_event_handler.cpp`：

```cpp
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
```

#### Step 4.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/chat/agent_event_handler_test.pro && make
```

Expected: `agent_event_handler.h: No such file or directory`。

#### Step 4.3 — Write the minimal implementation

`hermind-desktop/chat/agent_event_handler.h`：

```cpp
#ifndef AGENT_EVENT_HANDLER_H
#define AGENT_EVENT_HANDLER_H

#include <QObject>
#include <QVector>
#include "hermind_chat_message.h"
#include "hermind_agent_event.h"

class AgentEventHandler : public QObject
{
    Q_OBJECT
public:
    explicit AgentEventHandler(QObject *parent = nullptr);

    const QVector<HermindChatMessage> &messages() const;
    void clear();

    void handleEvent(const HermindAgentEvent &event);

signals:
    void messagesChanged();
    void statusReceived(const QString &status);
    void downloadCardReceived(const QJsonObject &payload);
    void visualizeChartReceived(const QJsonObject &payload);
    void toolApprovalRequested(const QString &requestId, const QString &skillName, const QString &description);
    void clarificationRequested(const QString &question);
    void errorReceived(const QString &error);
    void threadRenameRequested(const QString &newName);

private:
    QVector<HermindChatMessage> m_messages;
};

#endif // AGENT_EVENT_HANDLER_H
```

`hermind-desktop/chat/agent_event_handler.cpp`：

```cpp
#include "agent_event_handler.h"

AgentEventHandler::AgentEventHandler(QObject *parent)
    : QObject(parent)
{
}

const QVector<HermindChatMessage> &AgentEventHandler::messages() const
{
    return m_messages;
}

void AgentEventHandler::clear()
{
    m_messages.clear();
    emit messagesChanged();
}

void AgentEventHandler::handleEvent(const HermindAgentEvent &event)
{
    const QString type = event.type();

    if (type == QLatin1String("reportStreamEvent")) {
        HermindChatMessage msg;
        msg.setUuid(QString::number(qHash(event.content()))); // 简易 UUID；后续用真实 id
        msg.setRole(HermindChatMessage::Assistant);
        msg.setContent(event.content());
        msg.setClosed(true);
        m_messages.append(msg);
        emit messagesChanged();
    } else if (type == QLatin1String("statusResponse")) {
        emit statusReceived(event.content());
    } else if (type == QLatin1String("fileDownloadCard")) {
        emit downloadCardReceived(event.payload().toObject());
    } else if (type == QLatin1String("rechartVisualize")) {
        emit visualizeChartReceived(event.payload().toObject());
    } else if (type == QLatin1String("toolApprovalRequest")) {
        emit toolApprovalRequested(event.requestId(), event.skillName(), event.description());
    } else if (type == QLatin1String("clarificationRequest")) {
        emit clarificationRequested(event.question());
    } else if (type == QLatin1String("wssFailure")) {
        emit errorReceived(event.content());
    } else if (type == QLatin1String("action")) {
        const QString action = event.content();
        if (action == QLatin1String("rename_thread"))
            emit threadRenameRequested(event.payload().toObject().value("name").toString());
    }
}
```

#### Step 4.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/chat/agent_event_handler_test.pro && make && ./tests/chat/tst_agent_event_handler
```

Expected: `PASS`。

#### Step 4.5 — Commit

```bash
git add hermind-desktop/chat/agent_event_handler.* hermind-desktop/tests/chat/tst_agent_event_handler.cpp hermind-desktop/tests/chat/agent_event_handler_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add AgentEventHandler for websocket event aggregation"
```

---

## Local Self-Review (Stream Part)

- [ ] 1. Spec-coverage: SSE 事件处理 → Task 3；Agent WebSocket 事件处理 → Task 4。
- [ ] 2. Placeholder scan：无 TODO/TBD。
- [ ] 3. No phantom tasks：两个任务均产出文件、测试与 commit。
- [ ] 4. Dependency soundness：均依赖 Task 1 的 `HermindChatMessage`。
- [ ] 5. Caller & build soundness：新增类未修改现有签名；`.pro` 同步追加源文件。
- [ ] 6. Test-the-risk：Task 3 测试追加/关闭消息；Task 4 测试事件到信号的映射。
- [ ] 7. Type consistency：`HermindChatMessage` setter 在 Task 1 和 Task 3/4 间一致。
