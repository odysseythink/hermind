#include "hermind_api_client.h"

#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QJsonDocument>
#include <QJsonArray>

HermindApiClient::HermindApiClient(QObject *parent)
    : QObject(parent)
    , m_manager(new QNetworkAccessManager(this))
    , m_baseUrl(QStringLiteral("http://localhost:3001/api"))
{
}

void HermindApiClient::setBaseUrl(const QUrl &url) { m_baseUrl = url; }
QUrl HermindApiClient::baseUrl() const { return m_baseUrl; }
void HermindApiClient::setAuthToken(const QString &token) { m_authToken = token; }
QString HermindApiClient::authToken() const { return m_authToken; }

void HermindApiClient::requestToken(const QString &username,
                                    const QString &password,
                                    TokenCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("username"), username);
    body.insert(QStringLiteral("password"), password);

    post(QStringLiteral("/request-token"), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(QString(), QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             const bool valid = obj.value(QStringLiteral("valid")).toBool();
             const QString token = obj.value(QStringLiteral("token")).toString();
             const QString message = obj.value(QStringLiteral("message")).toString();

             if (!valid) {
                 ApiError err(message.isEmpty() ? QStringLiteral("Login failed") : message,
                              resp.statusCode());
                 callback(QString(), message, err);
                 return;
             }
             callback(token, message, ApiError());
         });
}

void HermindApiClient::refreshUser(UserCallback callback)
{
    get(QStringLiteral("/system/refresh-user"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(HermindUser(), QString(), resp.error());
                return;
            }
            const QJsonObject obj = resp.body().object();
            const bool success = obj.value(QStringLiteral("success")).toBool();
            const QString message = obj.value(QStringLiteral("message")).toString();

            if (!success) {
                callback(HermindUser(), message,
                         ApiError(message.isEmpty() ? QStringLiteral("Session invalid") : message,
                                  resp.statusCode()));
                return;
            }

            const QJsonValue userVal = obj.value(QStringLiteral("user"));
            if (userVal.isNull() || !userVal.isObject()) {
                callback(HermindUser(), QString(), ApiError());
                return;
            }
            callback(HermindUser::fromJson(userVal.toObject()), QString(), ApiError());
        });
}
void HermindApiClient::listWorkspaces(WorkspacesCallback callback)
{
    get(QStringLiteral("/workspaces"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindWorkspace>(), resp.error());
                return;
            }
            QVector<HermindWorkspace> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("workspaces")).toArray();
            list.reserve(arr.size());
            for (const QJsonValue &v : arr)
                list.append(HermindWorkspace::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}

void HermindApiClient::getWorkspace(const QString &slug, WorkspaceCallback callback)
{
    get(QStringLiteral("/workspace/") + slug, QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(HermindWorkspace(), QString(), resp.error());
                return;
            }
            const HermindWorkspace ws = HermindWorkspace::fromJson(
                resp.body().object().value(QStringLiteral("workspace")).toObject());
            callback(ws, QString(), ApiError());
        });
}

void HermindApiClient::createWorkspace(const QString &name, WorkspaceCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("name"), name);
    post(QStringLiteral("/workspace/new"), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(HermindWorkspace(), QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             const HermindWorkspace ws = HermindWorkspace::fromJson(
                 obj.value(QStringLiteral("workspace")).toObject());
             const QString message = obj.value(QStringLiteral("message")).toString();
             callback(ws, message, ApiError());
         });
}

void HermindApiClient::listThreads(const QString &workspaceSlug, ThreadsCallback callback)
{
    get(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/threads"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindWorkspaceThread>(), resp.error());
                return;
            }
            QVector<HermindWorkspaceThread> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("threads")).toArray();
            list.reserve(arr.size());
            for (const QJsonValue &v : arr)
                list.append(HermindWorkspaceThread::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}

void HermindApiClient::createThread(const QString &workspaceSlug, ThreadCallback callback)
{
    post(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/thread/new"), QJsonObject(),
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 const QString msg = resp.body().object().value(QStringLiteral("error")).toString();
                 callback(HermindWorkspaceThread(), msg, resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             const HermindWorkspaceThread thread = HermindWorkspaceThread::fromJson(
                 obj.value(QStringLiteral("thread")).toObject());
             callback(thread, QString(), ApiError());
         });
}

void HermindApiClient::updateThread(const QString &workspaceSlug,
                                    const QString &threadSlug,
                                    const QString &name,
                                    ThreadCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("name"), name);
    post(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/thread/") + threadSlug
             + QStringLiteral("/update"),
         body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 const QString msg = resp.body().object().value(QStringLiteral("message")).toString();
                 callback(HermindWorkspaceThread(), msg, resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             const HermindWorkspaceThread thread = HermindWorkspaceThread::fromJson(
                 obj.value(QStringLiteral("thread")).toObject());
             callback(thread, QString(), ApiError());
         });
}

void HermindApiClient::deleteThread(const QString &workspaceSlug,
                                    const QString &threadSlug,
                                    ThreadOperationCallback callback)
{
    del(QStringLiteral("/workspace/") + workspaceSlug + QStringLiteral("/thread/") + threadSlug,
        QJsonObject(),
        [callback](const ApiResponse &resp) {
            callback(resp.isSuccess(), resp.error());
        });
}

void HermindApiClient::get(const QString &path, const QUrlQuery &query, GenericCallback callback)
{
    sendRequest(QStringLiteral("GET"), path, query, QJsonObject(), callback);
}

void HermindApiClient::post(const QString &path, const QJsonObject &body, GenericCallback callback)
{
    sendRequest(QStringLiteral("POST"), path, QUrlQuery(), body, callback);
}

void HermindApiClient::del(const QString &path, const QJsonObject &body, GenericCallback callback)
{
    sendRequest(QStringLiteral("DELETE"), path, QUrlQuery(), body, callback);
}

void HermindApiClient::sendRequest(const QString &method,
                                   const QString &path,
                                   const QUrlQuery &query,
                                   const QJsonObject &body,
                                   GenericCallback callback)
{
    QUrl url = m_baseUrl;
    url.setPath(url.path() + path);
    if (!query.isEmpty())
        url.setQuery(query);

    QNetworkRequest request(url);
    request.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
    if (!m_authToken.isEmpty()) {
        request.setRawHeader(QByteArrayLiteral("Authorization"),
                             QByteArrayLiteral("Bearer ") + m_authToken.toUtf8());
    }

    QNetworkReply *reply = nullptr;
    const QByteArray payload = QJsonDocument(body).toJson(QJsonDocument::Compact);

    if (method == QStringLiteral("GET")) {
        reply = m_manager->get(request);
    } else if (method == QStringLiteral("POST")) {
        reply = m_manager->post(request, payload);
    } else if (method == QStringLiteral("DELETE")) {
        reply = m_manager->sendCustomRequest(request, method.toUtf8(), payload);
    } else {
        callback(ApiResponse(0, QJsonDocument(),
                             ApiError(QStringLiteral("Unsupported HTTP method: ") + method)));
        return;
    }

    connect(reply, &QNetworkReply::finished, this, [reply, callback]() {
        const int statusCode = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        const QByteArray rawBody = readReplyBody(reply);

        ApiError error;
        QJsonDocument doc;

        if (reply->error() != QNetworkReply::NoError) {
            error = parseNetworkError(reply);
        } else {
            QJsonParseError parseErr;
            doc = QJsonDocument::fromJson(rawBody, &parseErr);
            if (parseErr.error != QJsonParseError::NoError) {
                error = ApiError(parseErr.errorString(), statusCode);
            } else if (statusCode < 200 || statusCode >= 300) {
                QString msg = doc.isObject()
                                  ? doc.object().value(QStringLiteral("error")).toString()
                                  : QString::fromUtf8(rawBody);
                if (msg.isEmpty())
                    msg = QStringLiteral("HTTP error %1").arg(statusCode);
                error = ApiError(msg, statusCode);
            }
        }

        callback(ApiResponse(statusCode, doc, error));
        reply->deleteLater();
    });
}

ApiError HermindApiClient::parseNetworkError(QNetworkReply *reply)
{
    const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    return ApiError(reply->errorString(), status, reply->error());
}

#include <QJsonArray>
#include <QUrlQuery>

QByteArray HermindApiClient::readReplyBody(QNetworkReply *reply)
{
    return reply->readAll();
}

static QByteArray buildBearerHeader(const QString &token)
{
    if (token.isEmpty())
        return QByteArray();
    return QByteArrayLiteral("Bearer ") + token.toUtf8();
}

void HermindApiClient::streamChat(const QString &workspaceSlug,
                                  const QString &message,
                                  const QStringList &attachments,
                                  StreamChatCallback onChunk,
                                  std::function<void(const ApiError &)> onError,
                                  std::function<void()> onFinished)
{
    QUrl url = m_baseUrl;
    url.setPath(m_baseUrl.path() + QStringLiteral("/workspace/") + workspaceSlug
                + QStringLiteral("/stream-chat"));

    QJsonObject body;
    body.insert(QStringLiteral("message"), message);
    QJsonArray arr;
    for (const QString &a : attachments)
        arr.append(a);
    body.insert(QStringLiteral("attachments"), arr);

    if (!m_sseClient)
        m_sseClient = new HermindSseClient(m_manager, this);

    m_sseClient->start(url, body, buildBearerHeader(m_authToken),
                       std::move(onChunk), std::move(onError), std::move(onFinished));
}

void HermindApiClient::streamThreadChat(const QString &workspaceSlug,
                                        const QString &threadSlug,
                                        const QString &message,
                                        const QStringList &attachments,
                                        StreamChatCallback onChunk,
                                        std::function<void(const ApiError &)> onError,
                                        std::function<void()> onFinished)
{
    QUrl url = m_baseUrl;
    url.setPath(m_baseUrl.path() + QStringLiteral("/workspace/") + workspaceSlug
                + QStringLiteral("/thread/") + threadSlug
                + QStringLiteral("/stream-chat"));

    QJsonObject body;
    body.insert(QStringLiteral("message"), message);
    QJsonArray arr;
    for (const QString &a : attachments)
        arr.append(a);
    body.insert(QStringLiteral("attachments"), arr);

    if (!m_sseClient)
        m_sseClient = new HermindSseClient(m_manager, this);

    m_sseClient->start(url, body, buildBearerHeader(m_authToken),
                       std::move(onChunk), std::move(onError), std::move(onFinished));
}

void HermindApiClient::abortStream()
{
    if (m_sseClient)
        m_sseClient->stop();
}

void HermindApiClient::openAgentWebSocket(const QString &socketId,
                                          const QString &token,
                                          std::function<void(const QJsonObject &)> onMessage,
                                          std::function<void(const QString &)> onError,
                                          std::function<void()> onClosed,
                                          std::function<void()> onOpened)
{
    QUrl url = m_baseUrl;
    const bool isHttps = (url.scheme() == QStringLiteral("https"));
    url.setScheme(isHttps ? QStringLiteral("wss") : QStringLiteral("ws"));
    url.setPath(m_baseUrl.path() + QStringLiteral("/agent-invocation/") + socketId);

    QUrlQuery query;
    query.addQueryItem(QStringLiteral("token"), token);
    url.setQuery(query);

    if (!m_wsClient)
        m_wsClient = new HermindWebSocketClient(this);

    m_wsClient->open(url, std::move(onMessage), std::move(onError),
                     std::move(onClosed), std::move(onOpened));
}

void HermindApiClient::sendAgentFeedback(const QString &feedback,
                                         const QStringList &attachments)
{
    if (!m_wsClient)
        return;

    QJsonObject obj;
    obj.insert(QStringLiteral("type"), QStringLiteral("awaitingFeedback"));
    obj.insert(QStringLiteral("feedback"), feedback);
    QJsonArray arr;
    for (const QString &a : attachments)
        arr.append(a);
    obj.insert(QStringLiteral("attachments"), arr);

    m_wsClient->sendJson(obj);
}

void HermindApiClient::sendToolApprovalResponse(const QString &requestId, bool approved)
{
    if (!m_wsClient)
        return;

    QJsonObject obj;
    obj.insert(QStringLiteral("type"), QStringLiteral("toolApprovalResponse"));
    obj.insert(QStringLiteral("requestId"), requestId);
    obj.insert(QStringLiteral("approved"), approved);

    m_wsClient->sendJson(obj);
}

void HermindApiClient::closeAgentWebSocket()
{
    if (m_wsClient)
        m_wsClient->close();
}
