#ifndef CHAT_CONTAINER_WIDGET_H
#define CHAT_CONTAINER_WIDGET_H

#include <QWidget>
#include <memory>
#include "prompt_input.h"
#include "hermind_chat_message.h"
#include "hermind_stream_chat_response.h"
#include "hermind_agent_event.h"

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
    void sendCommand(const PromptCommand &command);
    void sendCommand(const QString &text, const QString &writeMode = QStringLiteral("replace"),
                     const QStringList &attachments = QStringList());

private slots:
    void loadHistory();
    void onStreamResponse(const HermindStreamChatResponse &response);
    void onAgentEvent(const HermindAgentEvent &event);
    void applyTheme();

private:
    void connectHandlers();
    void disconnectAgentSocket();
    void autoSubmit(const QString &text, const QStringList &attachments);

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    QString m_workspaceName;
    QString m_threadSlug;

    ChatHistoryWidget *m_historyWidget = nullptr;
    PromptInput *m_input = nullptr;

    std::unique_ptr<ChatStreamHandler> m_streamHandler;
    std::unique_ptr<AgentEventHandler> m_agentHandler;
    bool m_streaming = false;
};

#endif // CHAT_CONTAINER_WIDGET_H
