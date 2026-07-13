#include "chat_container_widget.h"
#include "hermind_api_client.h"
#include "hermind_workspace.h"
#include "chat_history_widget.h"
#include "chat_stream_handler.h"
#include "agent_event_handler.h"
#include "agent_status_banner.h"
#include "sources_sidebar.h"
#include "memories_sidebar.h"
#include "default_chat_widget.h"
#include "tool_approval_dialog.h"
#include "download_card.h"
#include "theme_manager.h"
#include "theme_colors.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QScrollBar>
#include <QStackedWidget>

ChatContainerWidget::ChatContainerWidget(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
    , m_streamHandler(std::make_unique<ChatStreamHandler>(this))
    , m_agentHandler(std::make_unique<AgentEventHandler>(this))
{
    setupThreePanelLayout();
    connectHandlers();
    applyTheme();

    // Start in welcome state
    showDefaultChat();
}

ChatContainerWidget::~ChatContainerWidget()
{
    disconnectAgentSocket();
}

void ChatContainerWidget::setupThreePanelLayout()
{
    QHBoxLayout *root = new QHBoxLayout(this);
    root->setContentsMargins(0, 0, 0, 0);
    root->setSpacing(0);

    // === LEFT PANEL: sources sidebar (partial — constructed, hidden) ===
    m_leftPanel = new QWidget(this);
    QVBoxLayout *leftLayout = new QVBoxLayout(m_leftPanel);
    leftLayout->setContentsMargins(0, 0, 0, 0);
    leftLayout->setSpacing(0);
    QPushButton *renameBtn = new QPushButton(QStringLiteral("重命名"), m_leftPanel);
    renameBtn->setFlat(true);
    leftLayout->addWidget(renameBtn);
    m_sourcesSidebar = new SourcesSidebar(m_leftPanel);
    leftLayout->addWidget(m_sourcesSidebar, 1);
    m_leftPanel->setFixedWidth(0); // hidden for now
    connect(renameBtn, &QPushButton::clicked, this, [this]() {
        emit requestThreadRename(m_threadSlug);
    });
    connect(m_sourcesSidebar, &SourcesSidebar::closeRequested, this, [this]() {
        m_leftPanel->setFixedWidth(0);
    });

    // === CENTER PANEL ===
    m_centerPanel = new QWidget(this);
    QVBoxLayout *centerLayout = new QVBoxLayout(m_centerPanel);
    centerLayout->setContentsMargins(0, 0, 0, 0);
    centerLayout->setSpacing(0);

    // Toolbar row
    QHBoxLayout *toolbar = new QHBoxLayout();
    toolbar->setContentsMargins(8, 4, 8, 4);
    m_wsLabel = new QLabel(m_centerPanel);
    m_wsLabel->setObjectName(QStringLiteral("workspaceNameLabel"));
    m_newChatBtn = new QPushButton(QStringLiteral("新建聊天"), m_centerPanel);
    m_stopBtn = new QPushButton(QStringLiteral("停止生成"), m_centerPanel);
    m_sourcesToggleBtn = new QPushButton(QStringLiteral("引用来源"), m_centerPanel);
    m_sourcesToggleBtn->setObjectName(QStringLiteral("sourcesToggleBtn"));
    m_memoriesToggleBtn = new QPushButton(QStringLiteral("记忆"), m_centerPanel);
    m_memoriesToggleBtn->setObjectName(QStringLiteral("memoriesToggleBtn"));
    m_newChatBtn->setFlat(true);
    m_stopBtn->setFlat(true);
    m_sourcesToggleBtn->setFlat(true);
    m_memoriesToggleBtn->setFlat(true);
    m_newChatBtn->setCursor(Qt::PointingHandCursor);
    m_stopBtn->setCursor(Qt::PointingHandCursor);
    m_sourcesToggleBtn->setCursor(Qt::PointingHandCursor);
    m_memoriesToggleBtn->setCursor(Qt::PointingHandCursor);
    toolbar->addWidget(m_wsLabel);
    toolbar->addStretch();
    toolbar->addWidget(m_sourcesToggleBtn);
    toolbar->addWidget(m_memoriesToggleBtn);
    toolbar->addWidget(m_newChatBtn);
    toolbar->addWidget(m_stopBtn);
    centerLayout->addLayout(toolbar);

    // Stacked widget: welcome page (DefaultChat) vs chat page (history + banner)
    m_stack = new QStackedWidget(m_centerPanel);
    m_stack->setObjectName(QStringLiteral("chatStack"));

    // Page 0: DefaultChat welcome page
    m_defaultChat = new DefaultChatWidget(m_apiClient, m_stack);
    m_stack->addWidget(m_defaultChat);

    // Page 1: chat page
    QWidget *chatPage = new QWidget(m_stack);
    QVBoxLayout *chatPageLayout = new QVBoxLayout(chatPage);
    chatPageLayout->setContentsMargins(0, 0, 0, 0);
    chatPageLayout->setSpacing(0);
    m_historyWidget = new ChatHistoryWidget(chatPage);
    m_statusBanner = new AgentStatusBanner(chatPage);
    chatPageLayout->addWidget(m_historyWidget, 1);
    chatPageLayout->addWidget(m_statusBanner);
    m_stack->addWidget(chatPage);

    centerLayout->addWidget(m_stack, 1);

    // Shared prompt input below the stack — usable in both welcome and chat views
    m_input = new PromptInput(m_centerPanel);
    m_input->setMaxHeight(200);
    if (m_apiClient) {
        m_input->setApiClient(m_apiClient);
        m_defaultChat->promptInput()->setApiClient(m_apiClient);
    }
    centerLayout->addWidget(m_input);

    // === RIGHT PANEL: memories sidebar (partial — constructed, hidden) ===
    m_memoriesSidebar = new MemoriesSidebar(m_apiClient, this);
    m_memoriesSidebar->setFixedWidth(0);
    connect(m_memoriesSidebar, &MemoriesSidebar::closeRequested, this, [this]() {
        m_memoriesSidebar->setFixedWidth(0);
    });

    root->addWidget(m_leftPanel);
    root->addWidget(m_centerPanel, 1);
    root->addWidget(m_memoriesSidebar);

    // Toolbar actions
    connect(m_newChatBtn, &QPushButton::clicked, this, &ChatContainerWidget::newChat);
    connect(m_stopBtn, &QPushButton::clicked, this, [this]() {
        if (m_apiClient)
            m_apiClient->abortStream();
    });
    connect(m_sourcesToggleBtn, &QPushButton::clicked, this, [this]() {
        if (m_sourcesSidebar->isOpen()) {
            m_sourcesSidebar->close(); // emits closeRequested -> panel collapses
            return;
        }
        // Populate from the most recent message that carries sources.
        QJsonArray latest;
        const QVector<HermindChatMessage> msgs = !m_streamHandler->messages().isEmpty()
            ? m_streamHandler->messages() : m_agentHandler->messages();
        for (int i = msgs.size() - 1; i >= 0; --i) {
            if (!msgs.at(i).sources().isEmpty()) {
                latest = msgs.at(i).sources();
                break;
            }
        }
        m_sourcesSidebar->setSources(latest);
        m_leftPanel->setFixedWidth(300);
        m_sourcesSidebar->open();
    });
    connect(m_memoriesToggleBtn, &QPushButton::clicked, this, [this]() {
        if (m_memoriesSidebar->isOpen()) {
            m_memoriesSidebar->close(); // emits closeRequested -> panel collapses
            return;
        }
        m_memoriesSidebar->setFixedWidth(300);
        m_memoriesSidebar->open();
    });

    // Shared PromptInput wiring
    connect(m_input, &PromptInput::sendCommand,
            this, QOverload<const PromptCommand &>::of(&ChatContainerWidget::sendCommand));
    connect(m_input, &PromptInput::stopRequested,
            this, [this]() { if (m_apiClient) m_apiClient->abortStream(); });

    // DefaultChatWidget wiring
    connect(m_defaultChat, &DefaultChatWidget::sendRequested,
            this, [this](const QString &text) { sendCommand(text); });
    connect(m_defaultChat->promptInput(), &PromptInput::sendCommand,
            this, QOverload<const PromptCommand &>::of(&ChatContainerWidget::sendCommand));
    connect(m_defaultChat->promptInput(), &PromptInput::stopRequested,
            this, [this]() { if (m_apiClient) m_apiClient->abortStream(); });
    connect(m_defaultChat, &DefaultChatWidget::createAgentClicked, this, []() {
        qDebug() << "Create agent clicked — navigation TBD";
    });
    connect(m_defaultChat, &DefaultChatWidget::editWorkspaceClicked, this, [this]() {
        qDebug() << "Edit workspace clicked for" << m_workspaceSlug;
    });
    connect(m_defaultChat, &DefaultChatWidget::uploadDocumentClicked, this, []() {
        qDebug() << "Upload document clicked";
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
}

void ChatContainerWidget::showDefaultChat()
{
    m_stack->setCurrentIndex(0);
}

void ChatContainerWidget::showChatState()
{
    m_stack->setCurrentIndex(1);
}

void ChatContainerWidget::newChat()
{
    if (m_apiClient)
        m_apiClient->abortStream();
    m_streamHandler->clear();
    m_agentHandler->clear();
    m_historyWidget->clear();
    m_threadSlug.clear();
    m_streaming = false;
    m_input->setStopVisible(false);
    m_sourcesSidebar->clear();
    m_sourcesSidebar->close();
    m_leftPanel->setFixedWidth(0);
    showDefaultChat();
}

void ChatContainerWidget::connectHandlers()
{
    connect(m_streamHandler.get(), &ChatStreamHandler::messagesChanged,
            this, &ChatContainerWidget::onHistoryChanged);
    connect(m_streamHandler.get(), &ChatStreamHandler::streamFinished,
            this, [this]() {
                m_streaming = false;
                m_input->setStopVisible(false);
                m_statusBanner->hideBanner();
                emit streamFinished();
            });
    connect(m_streamHandler.get(), &ChatStreamHandler::agentWebSocketRequested,
            this, [this](const QString &socketId, const QString &token) {
                if (m_apiClient)
                    m_apiClient->openAgentWebSocket(socketId, token,
                        [this](const QJsonObject &obj) { onAgentEvent(HermindAgentEvent::fromJson(obj)); },
                        [](const QString &err) { qWarning() << "agent ws error:" << err; },
                        [this]() { disconnectAgentSocket(); });
            });

    connect(m_agentHandler.get(), &AgentEventHandler::messagesChanged,
            this, &ChatContainerWidget::onHistoryChanged);

    connect(m_agentHandler.get(), &AgentEventHandler::citationsReceived,
            this, &ChatContainerWidget::showSources);
    connect(m_streamHandler.get(), &ChatStreamHandler::sourcesReceived,
            this, &ChatContainerWidget::showSources);

    connect(m_agentHandler.get(), &AgentEventHandler::statusReceived,
            m_statusBanner, &AgentStatusBanner::showStatus);
    connect(m_agentHandler.get(), &AgentEventHandler::clarificationRequested,
            m_statusBanner, &AgentStatusBanner::showClarification);
    connect(m_agentHandler.get(), &AgentEventHandler::errorReceived,
            this, [this](const QString &error) {
                m_statusBanner->showStatus(QStringLiteral("错误: %1").arg(error));
            });

    connect(m_agentHandler.get(), &AgentEventHandler::toolApprovalRequested,
            this, [this](const QString &requestId, const QString &skillName,
                         const QString &description) {
                ToolApprovalDialog *dlg = new ToolApprovalDialog(requestId, skillName, description, this);
                connect(dlg, &ToolApprovalDialog::approved, this, [this](const QString &rid) {
                    if (m_apiClient) m_apiClient->sendToolApprovalResponse(rid, true);
                });
                connect(dlg, &ToolApprovalDialog::rejected, this, [this](const QString &rid) {
                    if (m_apiClient) m_apiClient->sendToolApprovalResponse(rid, false);
                });
                dlg->setAttribute(Qt::WA_DeleteOnClose);
                dlg->show();
            });

    connect(m_agentHandler.get(), &AgentEventHandler::downloadCardReceived,
            this, [this](const QJsonObject &payload) {
                DownloadCard *card = new DownloadCard(payload, window());
                card->setAttribute(Qt::WA_DeleteOnClose);
                card->show();
            });

    connect(m_agentHandler.get(), &AgentEventHandler::threadRenameRequested,
            this, &ChatContainerWidget::requestThreadRename);
}

void ChatContainerWidget::showSources(const QString &, const QJsonArray &sources)
{
    m_sourcesSidebar->setSources(sources);
    m_leftPanel->setFixedWidth(300);
    m_sourcesSidebar->open();
}

void ChatContainerWidget::onHistoryChanged()
{
    QVector<HermindChatMessage> msgs = m_streamHandler->messages();
    if (msgs.isEmpty())
        msgs = m_agentHandler->messages();

    if (msgs.isEmpty()) {
        showDefaultChat();
    } else {
        showChatState();
        m_historyWidget->setMessages(msgs);
    }
}

void ChatContainerWidget::setWorkspace(const QString &slug, const QString &name)
{
    m_workspaceSlug = slug;
    m_workspaceName = name;
    m_historyWidget->setWelcomeText(tr("欢迎来到 %1").arg(name));
    m_defaultChat->setWorkspaceSlug(slug);
    m_input->setWorkspaceSlug(slug);
    m_defaultChat->promptInput()->setWorkspaceSlug(slug);

    if (m_apiClient && !slug.isEmpty()) {
        m_apiClient->getWorkspace(slug,
            [this, slug](const HermindWorkspace &ws, const QString &, const ApiError &err) {
                if (!err.isEmpty())
                    return;
                m_defaultChat->setSuggestedMessages(ws.suggestedMessages());
                m_memoriesSidebar->setWorkspace(slug, ws.id());
                m_wsLabel->setText(slug);
            });
    } else {
        m_wsLabel->setText(slug);
    }

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
    m_input->setText(text);
}

void ChatContainerWidget::sendCommand(const PromptCommand &command)
{
    sendCommand(command.text, command.writeMode, command.attachments);
}

void ChatContainerWidget::sendCommand(const QString &text, const QString &writeMode,
                                      const QStringList &attachments)
{
    if (writeMode == QStringLiteral("prepend")) {
        m_input->setText(text + QStringLiteral(" ") + m_input->text());
        return;
    }
    if (writeMode == QStringLiteral("append")) {
        m_input->setText(m_input->text() + text);
        return;
    }
    // "replace" mode — send directly
    showChatState();
    autoSubmit(text, attachments);
}

void ChatContainerWidget::autoSubmit(const QString &text, const QStringList &attachments)
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;
    if (text.isEmpty())
        return;

    HermindChatMessage userMsg;
    userMsg.setRole(HermindChatMessage::User);
    userMsg.setContent(text);
    QVector<HermindChatMessage> msgs = m_streamHandler->messages();
    msgs.append(userMsg);
    m_streamHandler->setMessages(msgs);

    m_input->clear();
    m_streaming = true;
    m_input->setStopVisible(true);
    emit streamStarted();

    auto onChunk = [this](const HermindStreamChatResponse &resp) { onStreamResponse(resp); };
    auto onError = [this](const ApiError &err) {
        qWarning() << "stream error:" << err.message();
        m_streaming = false;
        m_input->setStopVisible(false);
    };
    auto onFinished = [this]() {
        m_streamHandler->closeLastMessage();
        m_streaming = false;
        m_input->setStopVisible(false);
        emit streamFinished();
    };

    if (m_threadSlug.isEmpty())
        m_apiClient->streamChat(m_workspaceSlug, text, attachments, onChunk, onError, onFinished);
    else
        m_apiClient->streamThreadChat(m_workspaceSlug, m_threadSlug, text, attachments, onChunk, onError, onFinished);
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
