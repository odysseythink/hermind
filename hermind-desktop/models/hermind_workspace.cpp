#include "hermind_workspace.h"

#include <QJsonArray>

std::optional<QString> HermindWorkspace::optionalString(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toString();
}

std::optional<double> HermindWorkspace::optionalDouble(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toDouble();
}

std::optional<int> HermindWorkspace::optionalInt(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toInt();
}

std::optional<bool> HermindWorkspace::optionalBool(const QJsonObject &obj, const char *key)
{
    if (!obj.contains(key) || obj.value(key).isNull())
        return std::nullopt;
    return obj.value(key).toBool();
}

HermindWorkspace HermindWorkspace::fromJson(const QJsonObject &obj)
{
    HermindWorkspace ws;
    ws.m_id = obj.value("id").toInt();
    ws.m_name = obj.value("name").toString();
    ws.m_slug = obj.value("slug").toString();
    ws.m_vectorTag = optionalString(obj, "vectorTag");
    ws.m_createdAt = QDateTime::fromString(obj.value("createdAt").toString(), Qt::ISODate);
    ws.m_lastUpdatedAt = QDateTime::fromString(obj.value("lastUpdatedAt").toString(), Qt::ISODate);
    ws.m_openAiHistory = obj.value("openAiHistory").toInt(20);
    ws.m_openAiTemp = optionalDouble(obj, "openAiTemp");
    ws.m_openAiPrompt = optionalString(obj, "openAiPrompt");
    ws.m_similarityThreshold = optionalDouble(obj, "similarityThreshold");
    ws.m_chatProvider = optionalString(obj, "chatProvider");
    ws.m_chatModel = optionalString(obj, "chatModel");
    ws.m_topN = optionalInt(obj, "topN");
    ws.m_chatMode = obj.value("chatMode").toString(QStringLiteral("chat"));
    ws.m_pfpFilename = optionalString(obj, "pfpFilename");
    ws.m_agentProvider = optionalString(obj, "agentProvider");
    ws.m_agentModel = optionalString(obj, "agentModel");
    ws.m_queryRefusalResponse = optionalString(obj, "queryRefusalResponse");
    ws.m_vectorSearchMode = obj.value("vectorSearchMode").toString(QStringLiteral("default"));
    ws.m_compressEnabled = optionalBool(obj, "compressEnabled");
    ws.m_compressThreshold = optionalDouble(obj, "compressThreshold");
    ws.m_compressContextLen = optionalInt(obj, "compressContextLen");
    if (obj.contains(QStringLiteral("suggestedMessages"))) {
        const QJsonArray arr = obj.value(QStringLiteral("suggestedMessages")).toArray();
        for (const QJsonValue &v : arr)
            ws.m_suggestedMessages.append(v.toString());
    }
    return ws;
}

QJsonObject HermindWorkspace::toJson() const
{
    QJsonObject obj;
    obj.insert("id", m_id);
    obj.insert("name", m_name);
    obj.insert("slug", m_slug);
    obj.insert("vectorTag", m_vectorTag.has_value() ? QJsonValue(m_vectorTag.value()) : QJsonValue::Null);
    obj.insert("createdAt", m_createdAt.toString(Qt::ISODate));
    obj.insert("lastUpdatedAt", m_lastUpdatedAt.toString(Qt::ISODate));
    obj.insert("openAiHistory", m_openAiHistory);
    obj.insert("openAiTemp", m_openAiTemp.has_value() ? QJsonValue(m_openAiTemp.value()) : QJsonValue::Null);
    obj.insert("openAiPrompt", m_openAiPrompt.has_value() ? QJsonValue(m_openAiPrompt.value()) : QJsonValue::Null);
    obj.insert("similarityThreshold", m_similarityThreshold.has_value() ? QJsonValue(m_similarityThreshold.value()) : QJsonValue::Null);
    obj.insert("chatProvider", m_chatProvider.has_value() ? QJsonValue(m_chatProvider.value()) : QJsonValue::Null);
    obj.insert("chatModel", m_chatModel.has_value() ? QJsonValue(m_chatModel.value()) : QJsonValue::Null);
    obj.insert("topN", m_topN.has_value() ? QJsonValue(m_topN.value()) : QJsonValue::Null);
    obj.insert("chatMode", m_chatMode);
    obj.insert("pfpFilename", m_pfpFilename.has_value() ? QJsonValue(m_pfpFilename.value()) : QJsonValue::Null);
    obj.insert("agentProvider", m_agentProvider.has_value() ? QJsonValue(m_agentProvider.value()) : QJsonValue::Null);
    obj.insert("agentModel", m_agentModel.has_value() ? QJsonValue(m_agentModel.value()) : QJsonValue::Null);
    obj.insert("queryRefusalResponse", m_queryRefusalResponse.has_value() ? QJsonValue(m_queryRefusalResponse.value()) : QJsonValue::Null);
    obj.insert("vectorSearchMode", m_vectorSearchMode);
    obj.insert("compressEnabled", m_compressEnabled.has_value() ? QJsonValue(m_compressEnabled.value()) : QJsonValue::Null);
    obj.insert("compressThreshold", m_compressThreshold.has_value() ? QJsonValue(m_compressThreshold.value()) : QJsonValue::Null);
    obj.insert("compressContextLen", m_compressContextLen.has_value() ? QJsonValue(m_compressContextLen.value()) : QJsonValue::Null);
    return obj;
}

int HermindWorkspace::id() const { return m_id; }
QString HermindWorkspace::name() const { return m_name; }
QString HermindWorkspace::slug() const { return m_slug; }
std::optional<QString> HermindWorkspace::vectorTag() const { return m_vectorTag; }
QDateTime HermindWorkspace::createdAt() const { return m_createdAt; }
QDateTime HermindWorkspace::lastUpdatedAt() const { return m_lastUpdatedAt; }
int HermindWorkspace::openAiHistory() const { return m_openAiHistory; }
std::optional<double> HermindWorkspace::openAiTemp() const { return m_openAiTemp; }
std::optional<QString> HermindWorkspace::openAiPrompt() const { return m_openAiPrompt; }
std::optional<double> HermindWorkspace::similarityThreshold() const { return m_similarityThreshold; }
std::optional<QString> HermindWorkspace::chatProvider() const { return m_chatProvider; }
std::optional<QString> HermindWorkspace::chatModel() const { return m_chatModel; }
std::optional<int> HermindWorkspace::topN() const { return m_topN; }
QString HermindWorkspace::chatMode() const { return m_chatMode; }
std::optional<QString> HermindWorkspace::pfpFilename() const { return m_pfpFilename; }
std::optional<QString> HermindWorkspace::agentProvider() const { return m_agentProvider; }
std::optional<QString> HermindWorkspace::agentModel() const { return m_agentModel; }
std::optional<QString> HermindWorkspace::queryRefusalResponse() const { return m_queryRefusalResponse; }
QString HermindWorkspace::vectorSearchMode() const { return m_vectorSearchMode; }
std::optional<bool> HermindWorkspace::compressEnabled() const { return m_compressEnabled; }
std::optional<double> HermindWorkspace::compressThreshold() const { return m_compressThreshold; }
std::optional<int> HermindWorkspace::compressContextLen() const { return m_compressContextLen; }
QStringList HermindWorkspace::suggestedMessages() const { return m_suggestedMessages; }
void HermindWorkspace::setSuggestedMessages(const QStringList &msgs) { m_suggestedMessages = msgs; }
