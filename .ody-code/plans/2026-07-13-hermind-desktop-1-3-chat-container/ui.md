# Hermind Desktop 1:3 Chat Container — UI Widgets & Theme

**Scope:** 实现 `ChatMessageItem`、`ChatHistoryWidget`、`ChatContainerWidget` 三层 Qt Widget；接入 `ThemeManager` 与 `ThemeColors`。

**Part dependencies:**
- `2026-07-13-hermind-desktop-1-3-chat-container/models.md` (Task 1-2)
- `2026-07-13-hermind-desktop-1-3-chat-container/stream.md` (Task 3-4)

---

### Task 5: ChatMessageItem — 单条消息气泡

**Depends on:** Task 1

**Files:**
- Create: `hermind-desktop/widgets/chat_message_item.h`
- Create: `hermind-desktop/widgets/chat_message_item.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:37-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/widgets/chat_message_item_test.pro`
- Create: `hermind-desktop/tests/widgets/tst_chat_message_item.cpp`

**Rationale:** 每条消息用一个 QWidget 表示，根据 `role` 决定左右对齐与背景色。本阶段只做纯文本渲染。

#### Step 5.1 — Write the failing test

创建 `hermind-desktop/tests/widgets/tst_chat_message_item.cpp` 与 `hermind-desktop/tests/widgets/chat_message_item_test.pro`：

`chat_message_item_test.pro`：

```qmake
QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets $$PWD/../..

SOURCES += \
    tst_chat_message_item.cpp \
    ../../widgets/chat_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_chat_message.cpp

HEADERS += \
    ../../widgets/chat_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_chat_message.h
```

`tst_chat_message_item.cpp`：

```cpp
#include <QtTest>
#include <QApplication>
#include "chat_message_item.h"
#include "hermind_chat_message.h"

class TestChatMessageItem : public QObject
{
    Q_OBJECT
private slots:
    void userMessage_alignsRight();
    void assistantMessage_alignsLeft();
};

void TestChatMessageItem::userMessage_alignsRight()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::User);
    msg.setContent("Hello");

    ChatMessageItem item(nullptr);
    item.setMessage(msg);

    QCOMPARE(item.messageText(), QString("Hello"));
    QVERIFY(item.isUserMessage());
}

void TestChatMessageItem::assistantMessage_alignsLeft()
{
    HermindChatMessage msg;
    msg.setRole(HermindChatMessage::Assistant);
    msg.setContent("Hi there");

    ChatMessageItem item(nullptr);
    item.setMessage(msg);

    QVERIFY(!item.isUserMessage());
}

QTEST_MAIN(TestChatMessageItem)
#include "tst_chat_message_item.moc"
```

#### Step 5.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/widgets/chat_message_item_test.pro && make
```

Expected: `chat_message_item.h: No such file or directory`。

#### Step 5.3 — Write the minimal implementation

`hermind-desktop/widgets/chat_message_item.h`：

```cpp
#ifndef CHAT_MESSAGE_ITEM_H
#define CHAT_MESSAGE_ITEM_H

#include <QWidget>
#include <QLabel>
#include "hermind_chat_message.h"

class ChatMessageItem : public QWidget
{
    Q_OBJECT
public:
    explicit ChatMessageItem(QWidget *parent = nullptr);

    void setMessage(const HermindChatMessage &message);
    void setDarkMode(bool dark);

    QString messageText() const;
    bool isUserMessage() const;

private:
    void setupLayout();
    void applyStyle();

    HermindChatMessage m_message;
    QLabel *m_textLabel = nullptr;
    bool m_dark = false;
};

#endif // CHAT_MESSAGE_ITEM_H
```

`hermind-desktop/widgets/chat_message_item.cpp`：

```cpp
#include "chat_message_item.h"
#include "theme_colors.h"

#include <QHBoxLayout>
#include <QFrame>

ChatMessageItem::ChatMessageItem(QWidget *parent)
    : QWidget(parent)
{
    setupLayout();
    applyStyle();
}

void ChatMessageItem::setupLayout()
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 6, 16, 6);

    m_textLabel = new QLabel(this);
    m_textLabel->setWordWrap(true);
    m_textLabel->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_textLabel->setMaximumWidth(640);

    QFrame *bubble = new QFrame(this);
    bubble->setObjectName(QStringLiteral("bubbleFrame"));
    QHBoxLayout *bubbleLayout = new QHBoxLayout(bubble);
    bubbleLayout->setContentsMargins(12, 8, 12, 8);
    bubbleLayout->addWidget(m_textLabel);

    layout->addWidget(bubble);
}

void ChatMessageItem::setMessage(const HermindChatMessage &message)
{
    m_message = message;
    m_textLabel->setText(message.content());
    applyStyle();
}

void ChatMessageItem::setDarkMode(bool dark)
{
    m_dark = dark;
    applyStyle();
}

QString ChatMessageItem::messageText() const
{
    return m_textLabel->text();
}

bool ChatMessageItem::isUserMessage() const
{
    return m_message.role() == HermindChatMessage::User;
}

void ChatMessageItem::applyStyle()
{
    QHBoxLayout *layout = qobject_cast<QHBoxLayout *>(this->layout());
    if (!layout || layout->count() == 0)
        return;

    QFrame *bubble = qobject_cast<QFrame *>(layout->itemAt(0)->widget());
    if (!bubble)
        return;

    const bool user = isUserMessage();
    layout->setAlignment(user ? Qt::AlignRight : Qt::AlignLeft);

    const QColor bg = user ? ThemeColors::primary(m_dark)
                           : ThemeColors::cardBackground(m_dark);
    const QColor fg = user ? QColor(255, 255, 255)
                           : ThemeColors::textPrimary(m_dark);

    bubble->setStyleSheet(QStringLiteral(
        "#bubbleFrame { background-color: %1; border-radius: 12px; }"
    ).arg(bg.name()));
    m_textLabel->setStyleSheet(QStringLiteral("color: %1; font-size: 14px;").arg(fg.name()));
}
```

#### Step 5.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/widgets/chat_message_item_test.pro && make && ./tests/widgets/tst_chat_message_item
```

Expected: `PASS`。

#### Step 5.5 — Commit

```bash
git add hermind-desktop/widgets/chat_message_item.* hermind-desktop/tests/widgets/tst_chat_message_item.cpp hermind-desktop/tests/widgets/chat_message_item_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add ChatMessageItem widget"
```

---

### Task 6: ChatHistoryWidget — 可滚动消息列表与自动底部

**Depends on:** Task 1, Task 5

**Files:**
- Create: `hermind-desktop/widgets/chat_history_widget.h`
- Create: `hermind-desktop/widgets/chat_history_widget.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:37-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/widgets/chat_history_widget_test.pro`
- Create: `hermind-desktop/tests/widgets/tst_chat_history_widget.cpp`

**Rationale:** 消息列表需要垂直滚动；新消息到达时自动滚动到底部；空状态显示欢迎页。

#### Step 6.1 — Write the failing test

创建 `hermind-desktop/tests/widgets/tst_chat_history_widget.cpp` 与 `hermind-desktop/tests/widgets/chat_history_widget_test.pro`：

`chat_history_widget_test.pro`：

```qmake
QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets $$PWD/../..

SOURCES += \
    tst_chat_history_widget.cpp \
    ../../widgets/chat_history_widget.cpp \
    ../../widgets/chat_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../models/hermind_chat_message.cpp

HEADERS += \
    ../../widgets/chat_history_widget.h \
    ../../widgets/chat_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../models/hermind_chat_message.h
```

`tst_chat_history_widget.cpp`：

```cpp
#include <QtTest>
#include "chat_history_widget.h"
#include "hermind_chat_message.h"

class TestChatHistoryWidget : public QObject
{
    Q_OBJECT
private slots:
    void setMessages_createsItems();
    void appendMessage_scrollsToBottom();
};

void TestChatHistoryWidget::setMessages_createsItems()
{
    ChatHistoryWidget widget(nullptr);

    QVector<HermindChatMessage> msgs;
    HermindChatMessage m1;
    m1.setRole(HermindChatMessage::User);
    m1.setContent("Hi");
    msgs.append(m1);

    HermindChatMessage m2;
    m2.setRole(HermindChatMessage::Assistant);
    m2.setContent("Hello");
    msgs.append(m2);

    widget.setMessages(msgs);

    QCOMPARE(widget.messageCount(), 2);
}

void TestChatHistoryWidget::appendMessage_scrollsToBottom()
{
    ChatHistoryWidget widget(nullptr);
    widget.appendMessage([]{
        HermindChatMessage m;
        m.setRole(HermindChatMessage::Assistant);
        m.setContent("Bottom");
        return m;
    }());

    QCOMPARE(widget.messageCount(), 1);
    QVERIFY(widget.isAtBottom());
}

QTEST_MAIN(TestChatHistoryWidget)
#include "tst_chat_history_widget.moc"
```

#### Step 6.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/widgets/chat_history_widget_test.pro && make
```

Expected: `chat_history_widget.h: No such file or directory`。

#### Step 6.3 — Write the minimal implementation

`hermind-desktop/widgets/chat_history_widget.h`：

```cpp
#ifndef CHAT_HISTORY_WIDGET_H
#define CHAT_HISTORY_WIDGET_H

#include <QWidget>
#include <QScrollArea>
#include <QVBoxLayout>
#include <QVector>
#include "hermind_chat_message.h"

class ChatMessageItem;
class QLabel;

class ChatHistoryWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ChatHistoryWidget(QWidget *parent = nullptr);

    void setMessages(const QVector<HermindChatMessage> &messages);
    void appendMessage(const HermindChatMessage &message);
    void updateMessage(int index, const HermindChatMessage &message);
    void clear();

    int messageCount() const;
    bool isAtBottom() const;

    void setWelcomeText(const QString &text);

private:
    void rebuild();
    void appendItem(const HermindChatMessage &message);
    void scrollToBottom();

    QScrollArea *m_scrollArea = nullptr;
    QWidget *m_container = nullptr;
    QVBoxLayout *m_layout = nullptr;
    QLabel *m_welcomeLabel = nullptr;
    QVector<HermindChatMessage> m_messages;
    QVector<ChatMessageItem *> m_items;
};

#endif // CHAT_HISTORY_WIDGET_H
```

`hermind-desktop/widgets/chat_history_widget.cpp`：

```cpp
#include "chat_history_widget.h"
#include "chat_message_item.h"
#include "theme_colors.h"

#include <QScrollBar>
#include <QLabel>

ChatHistoryWidget::ChatHistoryWidget(QWidget *parent)
    : QWidget(parent)
{
    QVBoxLayout *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(0);

    m_welcomeLabel = new QLabel(tr("今天我能帮您什么？"), this);
    m_welcomeLabel->setObjectName(QStringLiteral("welcomeLabel"));
    m_welcomeLabel->setAlignment(Qt::AlignCenter);
    rootLayout->addWidget(m_welcomeLabel);

    m_scrollArea = new QScrollArea(this);
    m_scrollArea->setWidgetResizable(true);
    m_scrollArea->setFrameShape(QFrame::NoFrame);
    m_scrollArea->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_scrollArea->hide();

    m_container = new QWidget;
    m_layout = new QVBoxLayout(m_container);
    m_layout->setContentsMargins(0, 12, 0, 12);
    m_layout->setSpacing(4);
    m_layout->addStretch();

    m_scrollArea->setWidget(m_container);
    rootLayout->addWidget(m_scrollArea);
}

void ChatHistoryWidget::setMessages(const QVector<HermindChatMessage> &messages)
{
    m_messages = messages;
    rebuild();
}

void ChatHistoryWidget::appendMessage(const HermindChatMessage &message)
{
    m_messages.append(message);
    appendItem(message);
    scrollToBottom();
}

void ChatHistoryWidget::updateMessage(int index, const HermindChatMessage &message)
{
    if (index < 0 || index >= m_messages.size())
        return;
    m_messages[index] = message;
    if (index < m_items.size())
        m_items[index]->setMessage(message);
}

void ChatHistoryWidget::clear()
{
    m_messages.clear();
    rebuild();
}

int ChatHistoryWidget::messageCount() const
{
    return m_messages.size();
}

bool ChatHistoryWidget::isAtBottom() const
{
    QScrollBar *bar = m_scrollArea->verticalScrollBar();
    if (!bar)
        return true;
    return bar->value() >= bar->maximum() - 10;
}

void ChatHistoryWidget::setWelcomeText(const QString &text)
{
    m_welcomeLabel->setText(text);
}

void ChatHistoryWidget::rebuild()
{
    for (ChatMessageItem *item : m_items) {
        m_layout->removeWidget(item);
        item->deleteLater();
    }
    m_items.clear();

    const bool hasMessages = !m_messages.isEmpty();
    m_welcomeLabel->setVisible(!hasMessages);
    m_scrollArea->setVisible(hasMessages);

    // 保留底部的 stretch
    while (m_layout->count() > 0) {
        QLayoutItem *item = m_layout->takeAt(0);
        if (item->spacerItem())
            delete item;
    }

    for (const HermindChatMessage &msg : m_messages)
        appendItem(msg);

    m_layout->addStretch();
    scrollToBottom();
}

void ChatHistoryWidget::appendItem(const HermindChatMessage &message)
{
    ChatMessageItem *item = new ChatMessageItem(m_container);
    item->setMessage(message);
    m_items.append(item);
    m_layout->insertWidget(m_layout->count() - 1, item);
}

void ChatHistoryWidget::scrollToBottom()
{
    QScrollBar *bar = m_scrollArea->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}
```

#### Step 6.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/widgets/chat_history_widget_test.pro && make && ./tests/widgets/tst_chat_history_widget
```

Expected: `PASS`。

#### Step 6.5 — Commit

```bash
git add hermind-desktop/widgets/chat_history_widget.* hermind-desktop/tests/widgets/tst_chat_history_widget.cpp hermind-desktop/tests/widgets/chat_history_widget_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add ChatHistoryWidget with auto-scroll"
```

---

### Task 7: ChatContainerWidget — 聊天容器（状态 + API 调用）

**Depends on:** Task 1, Task 2, Task 3, Task 4, Task 6

**Files:**
- Create: `hermind-desktop/widgets/chat_container_widget.h`
- Create: `hermind-desktop/widgets/chat_container_widget.cpp`
- Modify: `hermind-desktop/hermind-desktop.pro:37-80`（SOURCES / HEADERS 追加）
- Create: `hermind-desktop/tests/widgets/chat_container_widget_test.pro`
- Create: `hermind-desktop/tests/widgets/tst_chat_container_widget.cpp`

**Rationale:** 容器持有 `HermindApiClient`、流处理器与 Agent 事件处理器，协调历史加载、发送消息、流式更新、停止生成。

#### Step 7.1 — Write the failing test

创建 `hermind-desktop/tests/widgets/tst_chat_container_widget.cpp` 与 `hermind-desktop/tests/widgets/chat_container_widget_test.pro`：

`chat_container_widget_test.pro`：

```qmake
QT += widgets testlib network websockets
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../api $$PWD/../../models $$PWD/../../chat $$PWD/../../streaming $$PWD/../..

SOURCES += \
    tst_chat_container_widget.cpp \
    ../../widgets/chat_container_widget.cpp \
    ../../widgets/chat_history_widget.cpp \
    ../../widgets/chat_message_item.cpp \
    ../../widgets/theme_colors.cpp \
    ../../theme_manager.cpp \
    ../../settings_store.cpp \
    ../../chat/chat_stream_handler.cpp \
    ../../chat/agent_event_handler.cpp \
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
    ../../widgets/chat_container_widget.h \
    ../../widgets/chat_history_widget.h \
    ../../widgets/chat_message_item.h \
    ../../widgets/theme_colors.h \
    ../../theme_manager.h \
    ../../settings_store.h \
    ../../chat/chat_stream_handler.h \
    ../../chat/agent_event_handler.h \
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

`tst_chat_container_widget.cpp`：

```cpp
#include <QtTest>
#include <QSignalSpy>
#include "chat_container_widget.h"
#include "hermind_api_client.h"

class TestChatContainerWidget : public QObject
{
    Q_OBJECT
private slots:
    void setWorkspace_updatesWelcomeLabel();
    void sendButtonClicked_startsStream();
};

void TestChatContainerWidget::setWorkspace_updatesWelcomeLabel()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    widget.setWorkspace("ws-1", "My Workspace");

    QCOMPARE(widget.workspaceSlug(), QString("ws-1"));
    QCOMPARE(widget.workspaceName(), QString("My Workspace"));
}

void TestChatContainerWidget::sendButtonClicked_startsStream()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace("ws-1", "My Workspace");
    widget.setInputText("Hello");

    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.onSendClicked();

    QCOMPARE(spy.count(), 1);
}

QTEST_MAIN(TestChatContainerWidget)
#include "tst_chat_container_widget.moc"
```

#### Step 7.2 — Run and verify FAILS

```bash
cd hermind-desktop && qmake tests/widgets/chat_container_widget_test.pro && make
```

Expected: `chat_container_widget.h: No such file or directory`。

#### Step 7.3 — Write the minimal implementation

`hermind-desktop/widgets/chat_container_widget.h`：

```cpp
#ifndef CHAT_CONTAINER_WIDGET_H
#define CHAT_CONTAINER_WIDGET_H

#include <QWidget>
#include <QLineEdit>
#include <QPushButton>
#include <memory>
#include "hermind_chat_message.h"

class HermindApiClient;
class ChatHistoryWidget;
class ChatStreamHandler;
class AgentEventHandler;

class ChatContainerWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ChatContainerWidget(HermindApiClient *apiClient, QWidget *parent = nullptr);
    ~ChatContainerWidget();

    void setWorkspace(const QString &slug, const QString &name);
    void setThreadSlug(const QString &threadSlug);

    QString workspaceSlug() const;
    QString workspaceName() const;
    QString threadSlug() const;

    void setInputText(const QString &text);

signals:
    void streamStarted();
    void streamFinished();
    void requestThreadRename(const QString &newName);

public slots:
    void onSendClicked();
    void onStopClicked();

private slots:
    void loadHistory();
    void onStreamResponse(const HermindStreamChatResponse &response);
    void onAgentEvent(const HermindAgentEvent &event);
    void applyTheme();

private:
    void connectHandlers();
    void disconnectAgentSocket();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    QString m_workspaceName;
    QString m_threadSlug;

    ChatHistoryWidget *m_historyWidget = nullptr;
    QLineEdit *m_inputEdit = nullptr;
    QPushButton *m_sendButton = nullptr;
    QPushButton *m_stopButton = nullptr;

    std::unique_ptr<ChatStreamHandler> m_streamHandler;
    std::unique_ptr<AgentEventHandler> m_agentHandler;
    bool m_streaming = false;
};

#endif // CHAT_CONTAINER_WIDGET_H
```

`hermind-desktop/widgets/chat_container_widget.cpp`：

```cpp
#include "chat_container_widget.h"
#include "hermind_api_client.h"
#include "hermind_stream_chat_response.h"
#include "hermind_agent_event.h"
#include "chat_history_widget.h"
#include "chat_stream_handler.h"
#include "agent_event_handler.h"
#include "theme_manager.h"
#include "theme_colors.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QScrollBar>

ChatContainerWidget::ChatContainerWidget(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
    , m_streamHandler(std::make_unique<ChatStreamHandler>(this))
    , m_agentHandler(std::make_unique<AgentEventHandler>(this))
{
    QVBoxLayout *root = new QVBoxLayout(this);
    root->setContentsMargins(0, 0, 0, 0);
    root->setSpacing(0);

    m_historyWidget = new ChatHistoryWidget(this);
    root->addWidget(m_historyWidget, 1);

    QHBoxLayout *inputLayout = new QHBoxLayout();
    m_inputEdit = new QLineEdit(this);
    m_inputEdit->setPlaceholderText(tr("发送消息"));
    m_sendButton = new QPushButton(tr("发送"), this);
    m_stopButton = new QPushButton(tr("停止"), this);
    m_stopButton->setVisible(false);

    inputLayout->addWidget(m_inputEdit, 1);
    inputLayout->addWidget(m_sendButton);
    inputLayout->addWidget(m_stopButton);
    root->addLayout(inputLayout);

    connect(m_sendButton, &QPushButton::clicked, this, &ChatContainerWidget::onSendClicked);
    connect(m_stopButton, &QPushButton::clicked, this, &ChatContainerWidget::onStopClicked);
    connect(m_inputEdit, &QLineEdit::returnPressed, this, &ChatContainerWidget::onSendClicked);
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ChatContainerWidget::applyTheme);

    connectHandlers();
    applyTheme();
}

ChatContainerWidget::~ChatContainerWidget() = default;

void ChatContainerWidget::connectHandlers()
{
    connect(m_streamHandler.get(), &ChatStreamHandler::messagesChanged,
            this, [this]() { m_historyWidget->setMessages(m_streamHandler->messages()); });
    connect(m_streamHandler.get(), &ChatStreamHandler::streamFinished,
            this, [this]() { m_streaming = false; m_stopButton->setVisible(false); emit streamFinished(); });
    connect(m_streamHandler.get(), &ChatStreamHandler::agentWebSocketRequested,
            this, [this](const QString &socketId, const QString &token) {
                if (m_apiClient)
                    m_apiClient->openAgentWebSocket(socketId, token,
                        [this](const QJsonObject &obj) { onAgentEvent(HermindAgentEvent::fromJson(obj)); },
                        [](const QString &err) { qWarning() << "agent ws error:" << err; },
                        [this]() { disconnectAgentSocket(); });
            });

    connect(m_agentHandler.get(), &AgentEventHandler::messagesChanged,
            this, [this]() { m_historyWidget->setMessages(m_agentHandler->messages()); });
}

void ChatContainerWidget::setWorkspace(const QString &slug, const QString &name)
{
    m_workspaceSlug = slug;
    m_workspaceName = name;
    m_historyWidget->setWelcomeText(tr("欢迎来到 %1").arg(name));
    loadHistory();
}

void ChatContainerWidget::setThreadSlug(const QString &threadSlug)
{
    m_threadSlug = threadSlug;
    loadHistory();
}

QString ChatContainerWidget::workspaceSlug() const { return m_workspaceSlug; }
QString ChatContainerWidget::workspaceName() const { return m_workspaceName; }
QString ChatContainerWidget::threadSlug() const { return m_threadSlug; }

void ChatContainerWidget::setInputText(const QString &text)
{
    m_inputEdit->setText(text);
}

void ChatContainerWidget::onSendClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    const QString text = m_inputEdit->text().trimmed();
    if (text.isEmpty())
        return;

    HermindChatMessage userMsg;
    userMsg.setRole(HermindChatMessage::User);
    userMsg.setContent(text);
    m_streamHandler->setMessages(m_streamHandler->messages() << userMsg);

    m_inputEdit->clear();
    m_streaming = true;
    m_stopButton->setVisible(true);
    emit streamStarted();

    auto onChunk = [this](const HermindStreamChatResponse &resp) { onStreamResponse(resp); };
    auto onError = [this](const ApiError &err) {
        qWarning() << "stream error:" << err.message();
        m_streaming = false;
        m_stopButton->setVisible(false);
    };
    auto onFinished = [this]() {
        m_streaming = false;
        m_stopButton->setVisible(false);
        emit streamFinished();
    };

    if (m_threadSlug.isEmpty())
        m_apiClient->streamChat(m_workspaceSlug, text, QStringList(), onChunk, onError, onFinished);
    else
        m_apiClient->streamThreadChat(m_workspaceSlug, m_threadSlug, text, QStringList(), onChunk, onError, onFinished);
}

void ChatContainerWidget::onStopClicked()
{
    if (m_apiClient)
        m_apiClient->abortStream();
}

void ChatContainerWidget::loadHistory()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    auto callback = [this](const QVector<HermindChatMessage> &msgs, const ApiError &err) {
        if (!err.isEmpty()) {
            qWarning() << "load history error:" << err.message();
            return;
        }
        m_streamHandler->setMessages(msgs);
    };

    if (m_threadSlug.isEmpty())
        m_apiClient->chatHistory(m_workspaceSlug, callback);
    else
        m_apiClient->threadChatHistory(m_workspaceSlug, m_threadSlug, callback);
}

void ChatContainerWidget::onStreamResponse(const HermindStreamChatResponse &response)
{
    m_streamHandler->handleResponse(response);
}

void ChatContainerWidget::onAgentEvent(const HermindAgentEvent &event)
{
    m_agentHandler->handleEvent(event);
}

void ChatContainerWidget::disconnectAgentSocket()
{
    if (m_apiClient)
        m_apiClient->closeAgentWebSocket();
}

void ChatContainerWidget::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    m_historyWidget->setStyleSheet(QStringLiteral("background-color: %1;").arg(ThemeColors::windowBackground(dark).name()));
}
```

> 注意：`HermindStreamChatResponse` 需要能被 `chat_container_widget.h` 使用；已在 `hermind_api_client.h` 中 include。若编译报错，在 `.cpp` 顶部 `#include "hermind_stream_chat_response.h"`。

#### Step 7.4 — Run and verify PASSES

```bash
cd hermind-desktop && qmake tests/widgets/chat_container_widget_test.pro && make && ./tests/widgets/tst_chat_container_widget
```

Expected: `PASS`。

#### Step 7.5 — Commit

```bash
git add hermind-desktop/widgets/chat_container_widget.* hermind-desktop/tests/widgets/tst_chat_container_widget.cpp hermind-desktop/tests/widgets/chat_container_widget_test.pro hermind-desktop/hermind-desktop.pro
git commit -m "feat(desktop): add ChatContainerWidget with stream and agent handling"
```

---

### Task 8: 主题样式集成

**Depends on:** Task 5, Task 6, Task 7

**Files:**
- Modify: `hermind-desktop/widgets/chat_message_item.cpp:60-120`（applyStyle 使用 ThemeColors）
- Modify: `hermind-desktop/widgets/chat_history_widget.cpp:30-40`（欢迎页颜色）
- Modify: `hermind-desktop/widgets/chat_container_widget.cpp:150-160`（applyTheme）

**Rationale:** 控件需在浅色/深色主题下正确显示，复用 `ThemeManager` 与 `ThemeColors`。

#### Step 8.1 — Add theme-aware styling to ChatHistoryWidget welcome label

在 `ChatHistoryWidget::ChatHistoryWidget` 构造函数中追加：

```cpp
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ChatHistoryWidget::applyTheme);
    applyTheme();
```

并新增私有方法：

```cpp
void ChatHistoryWidget::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    m_welcomeLabel->setStyleSheet(QStringLiteral(
        "#welcomeLabel { color: %1; font-size: 24px; font-weight: 500; }"
    ).arg(ThemeColors::textPrimary(dark).name()));
}
```

#### Step 8.2 — Ensure ChatMessageItem follows theme

`ChatMessageItem::setDarkMode` 已在构造函数外部可调用；在 `ChatHistoryWidget::appendItem` 中同步主题：

```cpp
void ChatHistoryWidget::appendItem(const HermindChatMessage &message)
{
    ChatMessageItem *item = new ChatMessageItem(m_container);
    item->setMessage(message);
    item->setDarkMode(ThemeManager::instance().isDarkMode());
    // ...
}
```

并在 `ChatHistoryWidget::applyTheme` 中遍历 `m_items` 调用 `setDarkMode`。

#### Step 8.3 — Build

```bash
cd hermind-desktop && qmake hermind-desktop.pro && make
```

Expected: build PASS。

#### Step 8.4 — Commit

```bash
git add hermind-desktop/widgets/chat_message_item.cpp hermind-desktop/widgets/chat_history_widget.cpp hermind-desktop/widgets/chat_container_widget.cpp
git commit -m "feat(desktop): wire chat widgets to ThemeManager"
```

---

## Local Self-Review (UI Part)

- [ ] 1. Spec-coverage: 单条消息 UI → Task 5；消息列表滚动 → Task 6；聊天容器状态 → Task 7；主题 → Task 8。
- [ ] 2. Placeholder scan：无 TODO/TBD。
- [ ] 3. No phantom tasks：四个任务均产出文件/测试/commit。
- [ ] 4. Dependency soundness：Task 5 依赖 Task 1；Task 6 依赖 Task 1/5；Task 7 依赖 Task 1/2/3/4/6；Task 8 依赖 Task 5/6/7。
- [ ] 5. Caller & build soundness：本 Part 未修改共享签名，新增类独立；Task 8 以完整 `make` 验证。
- [ ] 6. Test-the-risk：Task 5 测试角色与文本；Task 6 测试消息数量与滚动；Task 7 测试工作区设置与发送信号。
- [ ] 7. Type consistency：`HermindChatMessage` setter、`ThemeColors` 接口与 Part 1 一致。
