#include "chat_container_widget.h"
#include "hermind_api_client.h"
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

ChatContainerWidget::~ChatContainerWidget()
{
    disconnectAgentSocket();
}

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
    QVector<HermindChatMessage> msgs = m_streamHandler->messages();
    msgs.append(userMsg);
    m_streamHandler->setMessages(msgs);

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
