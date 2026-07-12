# Hermind Desktop 1:3 Chat Container — Integration, Build & Verification

**Scope:** 把 `ChatContainerWidget` 接入 `MainChatWidget`；更新 `hermind-desktop.pro`；运行整树构建、单测与手动端到端验证。

**Part dependencies:**
- `2026-07-13-hermind-desktop-1-3-chat-container/models.md` (Task 1-2)
- `2026-07-13-hermind-desktop-1-3-chat-container/stream.md` (Task 3-4)
- `2026-07-13-hermind-desktop-1-3-chat-container/ui.md` (Task 5-8)

---

### Task 9: MainChatWidget 替换占位聊天区

**Depends on:** Task 7, Task 8

**Files:**
- Modify: `hermind-desktop/main_chat_widget.h:46-54`（新增 `m_chatContainer` 与 `setupChatContainer`）
- Modify: `hermind-desktop/main_chat_widget.cpp:36-60` 与 `315-338`（构造与路由处理）
- Modify: `hermind-desktop/main_chat_widget.ui:225-481`（在 `chatFrame` 中预留 `chatContainerPlaceholder`）

**Rationale:** 当前 `MainChatWidget` 的聊天区是静态欢迎页 + 输入框；需要把 `ChatContainerWidget` 嵌入其中，并根据 `NavigationRoute` 切换工作区/线程。

#### Step 9.1 — Modify main_chat_widget.h

在 `private:` 区上方追加前向声明：

```cpp
class ChatContainerWidget;
```

在 `private:` 区追加成员：

```cpp
    void setupChatContainer();
    void onRouteChanged(const NavigationRoute &route); // 已存在，扩展实现

    Ui::MainChatWidget *ui;
    SidebarWidget *m_sidebar = nullptr;
    ChatContainerWidget *m_chatContainer = nullptr;
```

#### Step 9.2 — Modify main_chat_widget.ui

在 `chatFrame / verticalLayout_2` 中，把 `welcomeLabel`、`inputFrame` 等替换为单一占位 `QWidget`，objectName="chatContainerPlaceholder"。

简化做法：保留 `chatFrame`，但把 `verticalLayout_2` 的内容清空，仅保留一个 `QWidget` 占位。具体：
- 删除 `welcomeLabel`、`topChatSpacer`、`welcomeInputSpacer`、`inputFrame`、`inputActionSpacer`、`actionButtonsLayout`、`bottomChatSpacer`。
- 新增一个 `QWidget`，objectName="chatContainerPlaceholder"，sizePolicy 垂直 expanding。

#### Step 9.3 — Modify main_chat_widget.cpp

在文件顶部现有 include 之后追加：

```cpp
#include "widgets/chat_container_widget.h"
```

在构造函数中 `replaceSidebar()` 之后调用：

```cpp
    setupChatContainer();
```

新增方法：

```cpp
void MainChatWidget::setupChatContainer()
{
    if (!ui->chatContainerPlaceholder)
        return;

    m_chatContainer = new ChatContainerWidget(AuthManager::instance().apiClient(), this);
    m_chatContainer->setObjectName(QStringLiteral("chatContainerWidget"));

    QVBoxLayout *layout = qobject_cast<QVBoxLayout *>(ui->chatFrame->layout());
    if (layout) {
        int idx = -1;
        for (int i = 0; i < layout->count(); ++i) {
            if (layout->itemAt(i)->widget() == ui->chatContainerPlaceholder) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            layout->removeWidget(ui->chatContainerPlaceholder);
            layout->insertWidget(idx, m_chatContainer);
        }
    }

    ui->chatContainerPlaceholder->deleteLater();
    ui->chatContainerPlaceholder = nullptr;
}
```

修改 `onRouteChanged` 与 `updateSidebarSelection`，在切换路由时同步到 `m_chatContainer`：

```cpp
void MainChatWidget::onRouteChanged(const NavigationRoute &route)
{
    updateSidebarSelection(route);

    if (!m_chatContainer)
        return;

    switch (route.page) {
    case NavigationPage::WorkspaceChat:
        m_chatContainer->setWorkspace(route.workspaceSlug, route.workspaceSlug);
        m_chatContainer->setThreadSlug(route.threadSlug);
        break;
    default:
        m_chatContainer->setWorkspace(QString(), QString());
        m_chatContainer->setThreadSlug(QString());
        break;
    }
}
```

#### Step 9.4 — Build

```bash
cd hermind-desktop && qmake hermind-desktop.pro && make
```

Expected: build PASS。

#### Step 9.5 — Commit

```bash
git add hermind-desktop/main_chat_widget.* hermind-desktop/main_chat_widget.ui
git commit -m "feat(desktop): integrate ChatContainerWidget into MainChatWidget"
```

---

### Task 10: 更新 hermind-desktop.pro 并整树构建

**Depends on:** Task 9

**Files:**
- Modify: `hermind-desktop/hermind-desktop.pro:11-80`（追加所有新增源文件）

**Rationale:** 新模型、处理器、控件、测试文件必须加入 qmake 工程才能编译。

#### Step 10.1 — Verify .pro entries

确保 `hermind-desktop.pro` 包含：

```qmake
SOURCES += \
    # ... existing ...
    models/hermind_chat_message.cpp \
    chat/chat_stream_handler.cpp \
    chat/agent_event_handler.cpp \
    widgets/chat_message_item.cpp \
    widgets/chat_history_widget.cpp \
    widgets/chat_container_widget.cpp

HEADERS += \
    # ... existing ...
    models/hermind_chat_message.h \
    chat/chat_stream_handler.h \
    chat/agent_event_handler.h \
    widgets/chat_message_item.h \
    widgets/chat_history_widget.h \
    widgets/chat_container_widget.h

INCLUDEPATH += $$PWD $$PWD/api $$PWD/models $$PWD/chat $$PWD/streaming $$PWD/auth $$PWD/navigation $$PWD/widgets $$PWD/sidebar
```

#### Step 10.2 — Whole-tree build

```bash
cd hermind-desktop && qmake hermind-desktop.pro && make -j$(nproc)
```

Expected: 无编译/链接错误，生成 `release/hermind-desktop.exe`。

#### Step 10.3 — Verify no stale callers

搜索所有新增符号的引用：

```bash
cd hermind-desktop && grep -R "HermindChatMessage\|ChatStreamHandler\|AgentEventHandler\|ChatContainerWidget" --include="*.cpp" --include="*.h" .
```

Expected：所有引用都在本计划新增/修改的文件内。

#### Step 10.4 — Commit

```bash
git add hermind-desktop/hermind-desktop.pro
git commit -m "chore(desktop): update .pro for chat container sources"
```

---

### Task 11: 全量测试与手动端到端验证

**Depends on:** Task 10

**Files:**
- Create: `hermind-desktop/tests/run_all_tests.sh`（可选，汇总测试）
- Modify: `hermind-desktop/Makefile`（若存在，新增 `test` 目标）

**Rationale：** 单元测试保证回归；手动验证保证 UI 行为与后端一致。

#### Step 11.1 — Run unit tests

依次运行：

```bash
cd hermind-desktop
qmake tests/models/chat_message_test.pro && make && ./tests/models/tst_chat_message
qmake tests/api/chat_history_test.pro && make && ./tests/api/tst_api_client_chat_history
qmake tests/chat/chat_stream_handler_test.pro && make && ./tests/chat/tst_chat_stream_handler
qmake tests/chat/agent_event_handler_test.pro && make && ./tests/chat/tst_agent_event_handler
qmake tests/widgets/chat_message_item_test.pro && make && ./tests/widgets/tst_chat_message_item
qmake tests/widgets/chat_history_widget_test.pro && make && ./tests/widgets/tst_chat_history_widget
qmake tests/widgets/chat_container_widget_test.pro && make && ./tests/widgets/tst_chat_container_widget
```

Expected：全部 `PASS`。

#### Step 11.2 — Manual end-to-end verification

前置条件：
- 后端 `hermind` 已启动在 `http://localhost:3001`。
- 桌面应用已编译。

步骤与期望：
1. 启动 `release/hermind-desktop.exe`。
2. 登录或处于已登录状态（`AuthManager::instance().apiClient()` 非空）。
3. 在侧边栏点击某个工作区 → 观察 `MainChatWidget` 右侧聊天区从欢迎页切换为 `ChatContainerWidget`。
4. 在输入框输入 `你好`，点击发送 → 观察：
   - 用户消息立即出现在列表右侧。
   - 出现助手消息占位，并随 SSE `textResponseChunk` 逐步填充文本。
   - 收到 `finalizeResponseStream` 后消息停止更新。
5. 点击侧边栏的线程 → 观察 `ChatContainerWidget` 调用 `threadChatHistory` 并渲染历史消息。
6. 在助手生成过程中点击“停止”按钮 → 观察 `abortStream()` 被调用，当前消息标记为 closed。
7. 切换系统浅色/深色主题 → 观察消息气泡与欢迎页颜色随之变化。

#### Step 11.3 — Commit

```bash
git add hermind-desktop/tests/run_all_tests.sh hermind-desktop/Makefile
git commit -m "test(desktop): add chat container test runner and manual verification steps"
```

---

## Local Self-Review (Integration Part)

- [ ] 1. Spec-coverage: 替换主聊天区 → Task 9；工程更新与编译 → Task 10；全量测试 + 手动验证 → Task 11。
- [ ] 2. Placeholder scan：无 TODO/TBD。
- [ ] 3. No phantom tasks：三个任务均产出修改/脚本/验证步骤。
- [ ] 4. Dependency soundness：Task 9 依赖 Task 7/8；Task 10 依赖 Task 9；Task 11 依赖 Task 10。
- [ ] 5. Caller & build soundness：Task 9 修改 `MainChatWidget` 这一已有共享类，同任务内更新其头/实现/UI 与调用逻辑；Task 10 以完整 `make` 验证；Task 11 运行全部测试。
- [ ] 6. Test-the-risk：单元测试覆盖消息追加、历史加载、流停止等状态变更；手动验证覆盖 UI 与后端集成。
- [ ] 7. Type consistency：`ChatContainerWidget` 的 `setWorkspace`/`setThreadSlug` 签名与 `MainChatWidget` 调用处一致。
