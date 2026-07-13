#include "chat_container_widget.h"
#include "hermind_api_client.h"
#include "chat_history_widget.h"
#include "chat_stream_handler.h"
#include "agent_event_handler.h"
#include "agent_status_banner.h"
#include "tool_approval_dialog.h"
#include "download_card.h"
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

    m_statusBanner = new AgentStatusBanner(this);
    root->addWidget(m_statusBanner);

    m_input = new PromptInput(this);
    m_input->setMaxHeight(200);
    root->addWidget(m_input);

    connect(m_input, &PromptInput::sendCommand,
            this, QOverload<const PromptCommand &>::of(&ChatContainerWidget::sendCommand));
    connect(m_input, &PromptInput::stopRequested,
            this, [this]() { if (m_apiClient) m_apiClient->abortStream(); });
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
            this, [this]() { m_historyWidget->setMessages(m_agentHandler->messages()); });

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
                card->show(); // standalone for now; Task 9 will integrate into history
            });

    connect(m_agentHandler.get(), &AgentEventHandler::threadRenameRequested,
            this, &ChatContainerWidget::requestThreadRename);
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
