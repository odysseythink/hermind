#include "AppState.h"
#include "HermindClient.h"
#include "SSEParser.h"
#include <QNetworkReply>
#include <QJsonDocument>
#include <QDebug>

AppState::AppState(HermindClient *client, QObject *parent)
    : QObject(parent)
    , m_client(client)
    , m_sseParser(new SSEParser(this))
    , m_streamReply(nullptr)
{
    connect(m_sseParser, &SSEParser::eventReceived,
            this, [this](const QString &, const QString &data) {
        QJsonDocument doc = QJsonDocument::fromJson(data.toUtf8());
        QJsonObject obj = doc.object();
        QString type = obj.value("type").toString();
        if (type == "message_chunk") {
            QJsonObject payload = obj.value("data").toObject();
            QString text = payload.value("text").toString();
            if (!m_messages.isEmpty()) {
                QJsonObject last = m_messages.last().toObject();
                last["content"] = last.value("content").toString() + text;
                m_messages[m_messages.size() - 1] = last;
                emit messagesChanged();
            }
            emit streamChunk(text);
        } else if (type == "done") {
            m_isStreaming = false;
            emit isStreamingChanged();
            emit streamDone();
            if (m_streamReply) {
                m_streamReply->deleteLater();
                m_streamReply = nullptr;
            }
        } else if (type == "error") {
            m_isStreaming = false;
            emit isStreamingChanged();
            emit streamError(obj.value("data").toObject().value("message").toString());
            if (m_streamReply) {
                m_streamReply->deleteLater();
                m_streamReply = nullptr;
            }
        } else if (type == "tool_call") {
            QJsonObject payload = obj.value("data").toObject();
            // TODO: emit tool call state for UI
        }
    });
}

void AppState::boot()
{
    if (!m_client) return;
    m_client->get("/api/config/schema", [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            m_status = "error";
            m_flashMessage = error;
            emit statusChanged();
            emit flashMessageChanged();
            return;
        }
        m_configSections = resp.value("sections").toArray();
        emit configSectionsChanged();
        m_client->get("/api/config", [this](const QJsonObject &resp2, const QString &error2) {
            if (!error2.isEmpty()) {
                m_status = "error";
                m_flashMessage = error2;
                emit statusChanged();
                emit flashMessageChanged();
                return;
            }
            m_config = resp2.value("config").toObject();
            m_originalConfig = m_config;
            m_status = "ready";
            emit configChanged();
            emit originalConfigChanged();
            emit statusChanged();
        });
    });
}

void AppState::setActiveGroup(const QString &group)
{
    if (m_activeGroup == group) return;
    m_activeGroup = group;
    emit activeGroupChanged();
}

void AppState::setActiveSubKey(const QString &subKey)
{
    if (m_activeSubKey == subKey) return;
    m_activeSubKey = subKey;
    emit activeSubKeyChanged();
}

void AppState::setConfigField(const QString &section, const QString &field, const QJsonValue &value)
{
    QJsonObject sec = m_config.value(section).toObject();
    sec[field] = value;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::setConfigScalar(const QString &section, const QJsonValue &value)
{
    m_config[section] = value;
    emit configChanged();
    updateDirtyCount();
}

void AppState::setKeyedField(const QString &section, const QString &subkey,
                             const QString &instanceKey, const QString &field, const QJsonValue &value)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonObject container = sec.value(subkey).toObject();
    QJsonObject inst = container.value(instanceKey).toObject();
    inst[field] = value;
    container[instanceKey] = inst;
    sec[subkey] = container;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::createKeyedInstance(const QString &section, const QString &subkey,
                                   const QString &instanceKey, const QJsonObject &initial)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonObject container = sec.value(subkey).toObject();
    container[instanceKey] = initial;
    sec[subkey] = container;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::deleteKeyedInstance(const QString &section, const QString &subkey, const QString &instanceKey)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonObject container = sec.value(subkey).toObject();
    container.remove(instanceKey);
    sec[subkey] = container;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::setListField(const QString &section, const QString &subkey,
                            int index, const QString &field, const QJsonValue &value)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonArray arr = sec.value(subkey).toArray();
    QJsonObject item = arr.at(index).toObject();
    item[field] = value;
    arr[index] = item;
    sec[subkey] = arr;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::createListInstance(const QString &section, const QString &subkey, const QJsonObject &initial)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonArray arr = sec.value(subkey).toArray();
    arr.append(initial);
    sec[subkey] = arr;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::deleteListInstance(const QString &section, const QString &subkey, int index)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonArray arr = sec.value(subkey).toArray();
    arr.removeAt(index);
    sec[subkey] = arr;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::moveListInstance(const QString &section, const QString &subkey, int index, const QString &direction)
{
    QJsonObject sec = m_config.value(section).toObject();
    QJsonArray arr = sec.value(subkey).toArray();
    int newIndex = direction == "up" ? index - 1 : index + 1;
    if (newIndex < 0 || newIndex >= arr.size()) return;
    QJsonValue temp = arr.at(index);
    arr[index] = arr.at(newIndex);
    arr[newIndex] = temp;
    sec[subkey] = arr;
    m_config[section] = sec;
    emit configChanged();
    updateDirtyCount();
}

void AppState::saveConfig()
{
    if (!m_client) return;
    QJsonObject body;
    body["config"] = m_config;
    m_client->put("/api/config", body, [this](const QJsonObject &, const QString &error) {
        if (!error.isEmpty()) {
            emit toast("Save failed: " + error);
            return;
        }
        m_originalConfig = m_config;
        emit originalConfigChanged();
        updateDirtyCount();
        emit toast("Settings saved");
    });
}

void AppState::sendMessage(const QString &text)
{
    if (!m_client || text.isEmpty()) return;
    QJsonObject msg;
    msg["role"] = "user";
    msg["content"] = text;
    m_messages.append(msg);
    emit messagesChanged();

    QJsonObject body;
    body["user_message"] = text;
    m_client->post("/api/conversation/messages", body, [this](const QJsonObject &, const QString &error) {
        if (!error.isEmpty()) {
            emit toast("Failed to send: " + error);
            return;
        }
        startStream();
    });
}

void AppState::startStream()
{
    if (!m_client) return;
    QJsonObject assistantMsg;
    assistantMsg["role"] = "assistant";
    assistantMsg["content"] = "";
    m_messages.append(assistantMsg);
    emit messagesChanged();

    m_isStreaming = true;
    emit isStreamingChanged();
    m_streamReply = m_client->getStream("/api/sse");
    connect(m_streamReply, &QNetworkReply::readyRead, this, &AppState::onStreamReadyRead);
    connect(m_streamReply, &QNetworkReply::finished, this, &AppState::onStreamFinished);
}

void AppState::onStreamReadyRead()
{
    if (m_streamReply) {
        m_sseParser->feed(m_streamReply->readAll());
    }
}

void AppState::onStreamFinished()
{
    m_isStreaming = false;
    emit isStreamingChanged();
    if (m_streamReply) {
        m_streamReply->deleteLater();
        m_streamReply = nullptr;
    }
}

void AppState::cancelGeneration()
{
    if (m_streamReply) {
        m_streamReply->abort();
        m_streamReply->deleteLater();
        m_streamReply = nullptr;
    }
    m_isStreaming = false;
    emit isStreamingChanged();
}

void AppState::startNewSession()
{
    m_messages = QJsonArray();
    emit messagesChanged();
    setActiveGroup("");
    setActiveSubKey("");
}

void AppState::fetchProviderModels(const QString &instanceKey)
{
    if (!m_client) return;
    m_client->post("/api/providers/" + instanceKey + "/models", QJsonObject(),
                   [this, instanceKey](const QJsonObject &resp, const QString &) {
        QJsonArray models = resp.value("models").toArray();
        QJsonObject pm = m_providerModels;
        QJsonArray arr;
        for (const auto &v : models) arr.append(v);
        pm[instanceKey] = arr;
        m_providerModels = pm;
        emit providerModelsChanged();
    });
}

void AppState::testProvider(const QString &instanceKey)
{
    if (!m_client) return;
    m_client->post("/api/providers/" + instanceKey + "/test", QJsonObject(),
                   [this](const QJsonObject &resp, const QString &) {
        bool ok = resp.value("ok").toBool();
        int latency = resp.value("latency_ms").toInt();
        emit toast(ok ? QString("OK (%1ms)").arg(latency) : "Test failed");
    });
}

void AppState::testAuxiliary()
{
    if (!m_client) return;
    m_client->post("/api/auxiliary/test", QJsonObject(),
                   [this](const QJsonObject &resp, const QString &) {
        bool ok = resp.value("ok").toBool();
        int latency = resp.value("latency_ms").toInt();
        emit toast(ok ? QString("OK (%1ms)").arg(latency) : "Test failed");
    });
}

void AppState::fetchAuxiliaryModels()
{
    if (!m_client) return;
    m_client->post("/api/auxiliary/models", QJsonObject(),
                   [this](const QJsonObject &resp, const QString &) {
        QJsonArray models = resp.value("models").toArray();
        QJsonObject pm = m_providerModels;
        QJsonArray arr;
        for (const auto &v : models) arr.append(v);
        pm["__auxiliary__"] = arr;
        m_providerModels = pm;
        emit providerModelsChanged();
    });
}

void AppState::fetchFallbackModels(int index)
{
    if (!m_client) return;
    m_client->post("/api/fallback_providers/" + QString::number(index) + "/models", QJsonObject(),
                   [this, index](const QJsonObject &resp, const QString &) {
        QJsonArray models = resp.value("models").toArray();
        QJsonObject pm = m_providerModels;
        QJsonArray arr;
        for (const auto &v : models) arr.append(v);
        pm["__fallback_" + QString::number(index) + "__"] = arr;
        m_providerModels = pm;
        emit providerModelsChanged();
    });
}

void AppState::updateDirtyCount()
{
    int count = 0;
    const QStringList keys = m_config.keys();
    for (const QString &k : keys) {
        if (m_config.value(k) != m_originalConfig.value(k)) count++;
    }
    if (m_dirtyCount != count) {
        m_dirtyCount = count;
        emit dirtyCountChanged();
    }
}
