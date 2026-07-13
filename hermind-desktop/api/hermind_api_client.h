#ifndef HERMIND_API_CLIENT_H
#define HERMIND_API_CLIENT_H

#include <QObject>
#include <QUrl>
#include <QUrlQuery>
#include <QJsonObject>
#include <QVector>
#include <functional>

#include "api_response.h"
#include "hermind_user.h"
#include "hermind_workspace.h"
#include "hermind_workspace_thread.h"
#include "hermind_chat_message.h"
#include "hermind_stream_chat_response.h"
#include "hermind_memory.h"
#include "hermind_sse_client.h"
#include "hermind_websocket_client.h"

class QNetworkAccessManager;

class HermindApiClient : public QObject
{
    Q_OBJECT

public:
    using GenericCallback = std::function<void(const ApiResponse &response)>;
    using TokenCallback = std::function<void(const QString &token,
                                             const QString &message,
                                             const ApiError &error)>;
    using UserCallback = std::function<void(const HermindUser &user,
                                            const QString &message,
                                            const ApiError &error)>;
    using WorkspacesCallback = std::function<void(const QVector<HermindWorkspace> &workspaces,
                                                  const ApiError &error)>;
    using WorkspaceCallback = std::function<void(const HermindWorkspace &workspace,
                                                 const QString &message,
                                                 const ApiError &error)>;

    struct SearchResultWorkspace {
        QString slug;
        QString name;
    };
    struct SearchResultThread {
        QString slug;
        QString name;
        QString workspaceSlug;
        QString workspaceName;
    };
    struct SearchResults {
        QVector<SearchResultWorkspace> workspaces;
        QVector<SearchResultThread> threads;
    };
    using SearchCallback = std::function<void(const SearchResults &results,
                                              const ApiError &error)>;

    using ThreadsCallback = std::function<void(const QVector<HermindWorkspaceThread> &threads,
                                               const ApiError &error)>;
    using ThreadCallback = std::function<void(const HermindWorkspaceThread &thread,
                                              const QString &message,
                                              const ApiError &error)>;
    using ThreadOperationCallback = std::function<void(bool success, const ApiError &error)>;
    using StreamChatCallback = std::function<void(const HermindStreamChatResponse &response)>;

    using ChatHistoryCallback = std::function<void(const QVector<HermindChatMessage> &messages,
                                                   const ApiError &error)>;

    struct MemoriesResult {
        QVector<HermindMemory> workspace;
        QVector<HermindMemory> global;
    };
    using MemoriesCallback = std::function<void(const MemoriesResult &memories,
                                                const ApiError &error)>;
    using MemoryCallback = std::function<void(const HermindMemory &memory,
                                              const QString &message,
                                              const ApiError &error)>;
    using MemoryDeleteCallback = std::function<void(bool success, const ApiError &error)>;

    explicit HermindApiClient(QObject *parent = nullptr);

    void setBaseUrl(const QUrl &url);
    QUrl baseUrl() const;

    void setAuthToken(const QString &token);
    QString authToken() const;

    void requestToken(const QString &username,
                      const QString &password,
                      TokenCallback callback);
    void refreshUser(UserCallback callback);

    void listWorkspaces(WorkspacesCallback callback);
    void getWorkspace(const QString &slug, WorkspaceCallback callback);
    void createWorkspace(const QString &name, WorkspaceCallback callback);
    void searchWorkspaceOrThread(const QString &searchTerm, SearchCallback callback);

    void listThreads(const QString &workspaceSlug, ThreadsCallback callback);
    void createThread(const QString &workspaceSlug, ThreadCallback callback);
    void updateThread(const QString &workspaceSlug,
                      const QString &threadSlug,
                      const QString &name,
                      ThreadCallback callback);
    void deleteThread(const QString &workspaceSlug,
                      const QString &threadSlug,
                      ThreadOperationCallback callback);

    void chatHistory(const QString &workspaceSlug, ChatHistoryCallback callback);
    void threadChatHistory(const QString &workspaceSlug,
                           const QString &threadSlug,
                           ChatHistoryCallback callback);

    void streamChat(const QString &workspaceSlug,
                    const QString &message,
                    const QStringList &attachments,
                    StreamChatCallback onChunk,
                    std::function<void(const ApiError &)> onError,
                    std::function<void()> onFinished);

    void streamThreadChat(const QString &workspaceSlug,
                          const QString &threadSlug,
                          const QString &message,
                          const QStringList &attachments,
                          StreamChatCallback onChunk,
                          std::function<void(const ApiError &)> onError,
                          std::function<void()> onFinished);

    void abortStream();

    void openAgentWebSocket(const QString &socketId,
                            const QString &token,
                            std::function<void(const QJsonObject &)> onMessage,
                            std::function<void(const QString &)> onError,
                            std::function<void()> onClosed,
                            std::function<void()> onOpened = nullptr);

    void sendAgentFeedback(const QString &feedback,
                           const QStringList &attachments = QStringList());
    void sendToolApprovalResponse(const QString &requestId, bool approved);
    void closeAgentWebSocket();

    void get(const QString &path, const QUrlQuery &query, GenericCallback callback);
    void post(const QString &path, const QJsonObject &body, GenericCallback callback);
    void patch(const QString &path, const QJsonObject &body, GenericCallback callback);
    void del(const QString &path, const QJsonObject &body, GenericCallback callback);

    void listMemories(const QString &workspaceSlug, MemoriesCallback callback);
    void createMemory(int workspaceId,
                      const QString &content,
                      const QString &scope,
                      MemoryCallback callback);
    void updateMemory(int memoryId, const QString &content, MemoryCallback callback);
    void deleteMemory(int memoryId, MemoryDeleteCallback callback);
    void promoteMemory(int memoryId, MemoryCallback callback);
    void demoteMemory(int memoryId, int workspaceId, MemoryCallback callback);

private:
    void sendRequest(const QString &method,
                     const QString &path,
                     const QUrlQuery &query,
                     const QJsonObject &body,
                     GenericCallback callback);
    static ApiError parseNetworkError(QNetworkReply *reply);
    static QByteArray readReplyBody(QNetworkReply *reply);

    QNetworkAccessManager *m_manager = nullptr;
    QUrl m_baseUrl;
    QString m_authToken;
    HermindSseClient *m_sseClient = nullptr;
    HermindWebSocketClient *m_wsClient = nullptr;
};

#endif // HERMIND_API_CLIENT_H
