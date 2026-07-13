#include "hermind_api_client.h"

#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QHttpMultiPart>
#include <QFile>
#include <QFileInfo>
#include <QMimeDatabase>
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
             // Some endpoints report failures in the "error" field while still returning HTTP 200.
             const QString message = obj.value(QStringLiteral("message")).toString();
             const QString errorText = obj.value(QStringLiteral("error")).toString();
             const QString displayMessage = message.isEmpty() ? errorText : message;
             callback(ws, displayMessage, ApiError());
         });
}

void HermindApiClient::updateWorkspace(const QString &slug,
                                       const QJsonObject &fields,
                                       WorkspaceCallback callback)
{
    post(QStringLiteral("/workspace/") + slug + QStringLiteral("/update"),
         fields,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(HermindWorkspace(), QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             const QJsonValue wsVal = obj.value(QStringLiteral("workspace"));
             if (wsVal.isNull() || !wsVal.isObject()) {
                 const QString message = obj.value(QStringLiteral("message")).toString();
                 callback(HermindWorkspace(),
                          message,
                          ApiError(message, resp.statusCode()));
                 return;
             }
             callback(HermindWorkspace::fromJson(wsVal.toObject()),
                      obj.value(QStringLiteral("message")).toString(),
                      ApiError());
         });
}

void HermindApiClient::deleteWorkspace(const QString &slug,
                                       ThreadOperationCallback callback)
{
    del(QStringLiteral("/workspace/") + slug, QJsonObject(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(false, resp.error());
                return;
            }
            const QJsonObject obj = resp.body().object();
            callback(obj.value(QStringLiteral("success")).toBool(), ApiError());
        });
}

void HermindApiClient::searchWorkspaceOrThread(const QString &searchTerm, SearchCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("searchTerm"), searchTerm);
    post(QStringLiteral("/workspace/search"), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(SearchResults(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             SearchResults results;

             const QJsonArray wsArr = obj.value(QStringLiteral("workspaces")).toArray();
             results.workspaces.reserve(wsArr.size());
             for (const QJsonValue &v : wsArr) {
                 const QJsonObject o = v.toObject();
                 SearchResultWorkspace ws;
                 ws.slug = o.value(QStringLiteral("slug")).toString();
                 ws.name = o.value(QStringLiteral("name")).toString();
                 results.workspaces.append(ws);
             }

             const QJsonArray thArr = obj.value(QStringLiteral("threads")).toArray();
             results.threads.reserve(thArr.size());
             for (const QJsonValue &v : thArr) {
                 const QJsonObject o = v.toObject();
                 const QJsonObject wsObj = o.value(QStringLiteral("workspace")).toObject();
                 SearchResultThread th;
                 th.slug = o.value(QStringLiteral("slug")).toString();
                 th.name = o.value(QStringLiteral("name")).toString();
                 th.workspaceSlug = wsObj.value(QStringLiteral("slug")).toString();
                 th.workspaceName = wsObj.value(QStringLiteral("name")).toString();
                 results.threads.append(th);
             }

             callback(results, ApiError());
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

void HermindApiClient::uploadAndEmbedFile(const QString &workspaceSlug,
                                          const QString &filePath,
                                          DocumentCallback callback)
{
    QFile *file = new QFile(filePath);
    if (!file->open(QIODevice::ReadOnly)) {
        callback(QJsonObject(), ApiError(QStringLiteral("Cannot open file: ") + filePath));
        delete file;
        return;
    }

    QHttpMultiPart *multiPart = new QHttpMultiPart(QHttpMultiPart::FormDataType);
    QHttpPart filePart;
    filePart.setHeader(QNetworkRequest::ContentTypeHeader,
                       QVariant(QMimeDatabase().mimeTypeForFile(filePath).name()));
    filePart.setHeader(QNetworkRequest::ContentDispositionHeader,
                       QVariant(QStringLiteral("form-data; name=\"file\"; filename=\"%1\"")
                                    .arg(QFileInfo(filePath).fileName())));
    filePart.setBodyDevice(file);
    file->setParent(multiPart);
    multiPart->append(filePart);

    QUrl url = m_baseUrl;
    url.setPath(url.path() + QStringLiteral("/workspace/%1/upload-and-embed").arg(workspaceSlug));
    QNetworkRequest request(url);
    if (!m_authToken.isEmpty()) {
        request.setRawHeader(QByteArrayLiteral("Authorization"),
                             QByteArrayLiteral("Bearer ") + m_authToken.toUtf8());
    }

    QNetworkReply *reply = m_manager->post(request, multiPart);
    multiPart->setParent(reply);

    connect(reply, &QNetworkReply::finished, this, [reply, callback]() {
        const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        ApiError error;
        QJsonObject document;

        if (reply->error() != QNetworkReply::NoError) {
            error = parseNetworkError(reply);
        } else {
            QJsonParseError parseErr;
            const QJsonDocument doc = QJsonDocument::fromJson(readReplyBody(reply), &parseErr);
            if (parseErr.error != QJsonParseError::NoError) {
                error = ApiError(parseErr.errorString(), status);
            } else if (status < 200 || status >= 300) {
                QString msg = doc.object().value(QStringLiteral("error")).toString();
                if (msg.isEmpty())
                    msg = QStringLiteral("HTTP error %1").arg(status);
                error = ApiError(msg, status);
            } else {
                document = doc.object().value(QStringLiteral("document")).toObject();
            }
        }

        callback(document, error);
        reply->deleteLater();
    });
}

void HermindApiClient::removeAndUnembed(const QString &workspaceSlug,
                                        const QString &docId,
                                        ThreadOperationCallback callback)
{
    QUrlQuery query;
    query.addQueryItem(QStringLiteral("docId"), docId);
    sendRequest(QStringLiteral("DELETE"),
                QStringLiteral("/workspace/%1/remove-and-unembed").arg(workspaceSlug),
                query, QJsonObject(),
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

void HermindApiClient::patch(const QString &path, const QJsonObject &body, GenericCallback callback)
{
    sendRequest(QStringLiteral("PATCH"), path, QUrlQuery(), body, callback);
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
    } else if (method == QStringLiteral("PATCH")) {
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

void HermindApiClient::getSuggestedMessages(const QString &slug,
                                            SuggestedMessagesCallback callback)
{
    get(QStringLiteral("/workspace/") + slug + QStringLiteral("/suggested-messages"),
        QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QStringList(), resp.error());
                return;
            }
            QStringList list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("suggestedMessages")).toArray();
            for (const QJsonValue &v : arr)
                list.append(v.toString());
            callback(list, ApiError());
        });
}

void HermindApiClient::setSuggestedMessages(const QString &slug,
                                            const QStringList &messages,
                                            OperationCallback callback)
{
    QJsonObject body;
    QJsonArray arr;
    for (const QString &m : messages)
        arr.append(m);
    body.insert(QStringLiteral("messages"), arr);

    post(QStringLiteral("/workspace/") + slug + QStringLiteral("/suggested-messages"),
         body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(false, QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             callback(obj.value(QStringLiteral("success")).toBool(),
                      obj.value(QStringLiteral("error")).toString(),
                      ApiError());
         });
}

void HermindApiClient::systemKeys(SystemKeysCallback callback)
{
    get(QStringLiteral("/setup-complete"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QJsonObject(), resp.error());
                return;
            }
            callback(resp.body().object().value(QStringLiteral("results")).toObject(), ApiError());
        });
}

void HermindApiClient::systemVectors(const QString &slug, SystemVectorsCallback callback)
{
    QUrlQuery query;
    query.addQueryItem(QStringLiteral("slug"), slug);
    get(QStringLiteral("/system/system-vectors"), query,
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(0, resp.error());
                return;
            }
            callback(static_cast<int>(resp.body().object().value(QStringLiteral("vectorCount")).toDouble()),
                     ApiError());
        });
}

void HermindApiClient::customModels(const QString &provider, CustomModelsCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("provider"), provider);
    post(QStringLiteral("/system/custom-models"), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(QStringList(), resp.error());
                 return;
             }
             QStringList ids;
             const QJsonArray arr = resp.body().object().value(QStringLiteral("models")).toArray();
             for (const QJsonValue &v : arr) {
                 if (v.isObject())
                     ids.append(v.toObject().value(QStringLiteral("id")).toString());
                 else
                     ids.append(v.toString());
             }
             callback(ids, ApiError());
         });
}

void HermindApiClient::updateSystemEnv(const QJsonObject &env, SystemKeysCallback callback)
{
    post(QStringLiteral("/system/update-env"), env,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(QJsonObject(), resp.error());
                 return;
             }
             callback(resp.body().object().value(QStringLiteral("newValues")).toObject(), ApiError());
         });
}

void HermindApiClient::updateSystemPreferences(const QJsonObject &prefs, OperationCallback callback)
{
    post(QStringLiteral("/admin/system-preferences"), prefs,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(false, QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             callback(obj.value(QStringLiteral("success")).toBool(),
                      obj.value(QStringLiteral("error")).toString(),
                      ApiError());
         });
}

void HermindApiClient::defaultSystemPrompt(DefaultSystemPromptCallback callback)
{
    get(QStringLiteral("/system/default-system-prompt"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QString(), resp.error());
                return;
            }
            callback(resp.body().object().value(QStringLiteral("defaultSystemPrompt")).toString(), ApiError());
        });
}

void HermindApiClient::promptVariables(PromptVariablesCallback callback)
{
    get(QStringLiteral("/system/prompt-variables"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QJsonArray(), resp.error());
                return;
            }
            callback(resp.body().object().value(QStringLiteral("variables")).toArray(), ApiError());
        });
}

void HermindApiClient::listUsers(UsersCallback callback)
{
    get(QStringLiteral("/admin/users"), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindUser>(), resp.error());
                return;
            }
            QVector<HermindUser> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("users")).toArray();
            for (const QJsonValue &v : arr)
                list.append(HermindUser::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}

void HermindApiClient::listWorkspaceUsers(int workspaceId, WorkspaceUsersCallback callback)
{
    get(QStringLiteral("/admin/workspaces/%1/users").arg(workspaceId), QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(QVector<HermindWorkspaceUser>(), resp.error());
                return;
            }
            QVector<HermindWorkspaceUser> list;
            const QJsonArray arr = resp.body().object().value(QStringLiteral("users")).toArray();
            for (const QJsonValue &v : arr)
                list.append(HermindWorkspaceUser::fromJson(v.toObject()));
            callback(list, ApiError());
        });
}

void HermindApiClient::updateWorkspaceUsers(int workspaceId,
                                            const QVector<int> &userIds,
                                            OperationCallback callback)
{
    QJsonObject body;
    QJsonArray arr;
    for (int id : userIds)
        arr.append(id);
    body.insert(QStringLiteral("userIds"), arr);
    post(QStringLiteral("/admin/workspaces/%1/update-users").arg(workspaceId), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(false, QString(), resp.error());
                 return;
             }
             const QJsonObject obj = resp.body().object();
             callback(obj.value(QStringLiteral("success")).toBool(),
                      obj.value(QStringLiteral("error")).toString(),
                      ApiError());
         });
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

void HermindApiClient::listMemories(const QString &workspaceSlug, MemoriesCallback callback)
{
    get(QStringLiteral("/memory/workspace/") + workspaceSlug, QUrlQuery(),
        [callback](const ApiResponse &resp) {
            if (!resp.isSuccess()) {
                callback(MemoriesResult(), resp.error());
                return;
            }
            MemoriesResult result;
            const QJsonObject obj = resp.body().object();
            const QJsonArray wsArr = obj.value(QStringLiteral("workspace")).toArray();
            const QJsonArray glArr = obj.value(QStringLiteral("global")).toArray();
            result.workspace.reserve(wsArr.size());
            for (const QJsonValue &v : wsArr)
                result.workspace.append(HermindMemory::fromJson(v.toObject()));
            result.global.reserve(glArr.size());
            for (const QJsonValue &v : glArr)
                result.global.append(HermindMemory::fromJson(v.toObject()));
            callback(result, ApiError());
        });
}

void HermindApiClient::createMemory(int workspaceId,
                                    const QString &content,
                                    const QString &scope,
                                    MemoryCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("content"), content);
    body.insert(QStringLiteral("scope"), scope);
    if (scope == QStringLiteral("global"))
        body.insert(QStringLiteral("workspaceId"), QJsonValue());
    else
        body.insert(QStringLiteral("workspaceId"), workspaceId);

    post(QStringLiteral("/memory"), body,
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(HermindMemory(),
                          resp.body().object().value(QStringLiteral("error")).toString(),
                          resp.error());
                 return;
             }
             const HermindMemory mem = HermindMemory::fromJson(
                 resp.body().object().value(QStringLiteral("memory")).toObject());
             callback(mem, QString(), ApiError());
         });
}

void HermindApiClient::updateMemory(int memoryId, const QString &content, MemoryCallback callback)
{
    QJsonObject body;
    body.insert(QStringLiteral("content"), content);
    patch(QStringLiteral("/memory/") + QString::number(memoryId), body,
          [callback](const ApiResponse &resp) {
              if (!resp.isSuccess()) {
                  callback(HermindMemory(),
                           resp.body().object().value(QStringLiteral("error")).toString(),
                           resp.error());
                  return;
              }
              const HermindMemory mem = HermindMemory::fromJson(
                  resp.body().object().value(QStringLiteral("memory")).toObject());
              callback(mem, QString(), ApiError());
          });
}

void HermindApiClient::deleteMemory(int memoryId, MemoryDeleteCallback callback)
{
    del(QStringLiteral("/memory/") + QString::number(memoryId), QJsonObject(),
        [callback](const ApiResponse &resp) {
            callback(resp.isSuccess(), resp.error());
        });
}

void HermindApiClient::promoteMemory(int memoryId, MemoryCallback callback)
{
    post(QStringLiteral("/memory/") + QString::number(memoryId) + QStringLiteral("/promote"),
         QJsonObject(),
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(HermindMemory(),
                          resp.body().object().value(QStringLiteral("error")).toString(),
                          resp.error());
                 return;
             }
             const HermindMemory mem = HermindMemory::fromJson(
                 resp.body().object().value(QStringLiteral("memory")).toObject());
             callback(mem, QString(), ApiError());
         });
}

void HermindApiClient::demoteMemory(int memoryId, int workspaceId, MemoryCallback callback)
{
    post(QStringLiteral("/memory/") + QString::number(memoryId) + QStringLiteral("/demote/")
             + QString::number(workspaceId),
         QJsonObject(),
         [callback](const ApiResponse &resp) {
             if (!resp.isSuccess()) {
                 callback(HermindMemory(),
                          resp.body().object().value(QStringLiteral("error")).toString(),
                          resp.error());
                 return;
             }
             const HermindMemory mem = HermindMemory::fromJson(
                 resp.body().object().value(QStringLiteral("memory")).toObject());
             callback(mem, QString(), ApiError());
         });
}
