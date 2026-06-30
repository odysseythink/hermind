#include "hermind_stream_chat_response.h"

std::optional<QString> HermindStreamChatResponse::optionalString(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toString();
}

std::optional<int> HermindStreamChatResponse::optionalInt(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toInt();
}

HermindStreamChatResponse HermindStreamChatResponse::fromJson(const QJsonObject &obj)
{
    HermindStreamChatResponse r;
    r.m_raw = obj;
    r.m_uuid = obj.value("uuid").toString();
    r.m_type = obj.value("type").toString();
    r.m_textResponse = optionalString(obj, "textResponse");
    r.m_sources = obj.value("sources").toArray();
    r.m_close = obj.value("close").toBool();
    r.m_error = optionalString(obj, "error");
    r.m_animate = obj.value("animate").toBool();
    r.m_chatId = optionalInt(obj, "chatId");
    r.m_action = optionalString(obj, "action");
    r.m_websocketUUID = optionalString(obj, "websocketUUID");
    r.m_websocketToken = optionalString(obj, "websocketToken");
    r.m_routedTo = optionalString(obj, "routedTo");
    r.m_metrics = obj.value("metrics").toObject();
    return r;
}

QJsonObject HermindStreamChatResponse::toJson() const
{
    return m_raw;
}

QString HermindStreamChatResponse::uuid() const { return m_uuid; }
QString HermindStreamChatResponse::type() const { return m_type; }
std::optional<QString> HermindStreamChatResponse::textResponse() const { return m_textResponse; }
QJsonArray HermindStreamChatResponse::sources() const { return m_sources; }
bool HermindStreamChatResponse::close() const { return m_close; }
std::optional<QString> HermindStreamChatResponse::error() const { return m_error; }
bool HermindStreamChatResponse::animate() const { return m_animate; }
std::optional<int> HermindStreamChatResponse::chatId() const { return m_chatId; }
std::optional<QString> HermindStreamChatResponse::action() const { return m_action; }
std::optional<QString> HermindStreamChatResponse::websocketUUID() const { return m_websocketUUID; }
std::optional<QString> HermindStreamChatResponse::websocketToken() const { return m_websocketToken; }
std::optional<QString> HermindStreamChatResponse::routedTo() const { return m_routedTo; }
QJsonObject HermindStreamChatResponse::metrics() const { return m_metrics; }
QJsonObject HermindStreamChatResponse::raw() const { return m_raw; }
