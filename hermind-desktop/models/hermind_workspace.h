#ifndef HERMIND_WORKSPACE_H
#define HERMIND_WORKSPACE_H

#include <QString>
#include <QJsonObject>
#include <QDateTime>
#include <optional>

class HermindWorkspace
{
public:
    HermindWorkspace() = default;

    static HermindWorkspace fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    int id() const;
    QString name() const;
    QString slug() const;
    std::optional<QString> vectorTag() const;
    QDateTime createdAt() const;
    QDateTime lastUpdatedAt() const;
    int openAiHistory() const;
    std::optional<double> openAiTemp() const;
    std::optional<QString> openAiPrompt() const;
    std::optional<double> similarityThreshold() const;
    std::optional<QString> chatProvider() const;
    std::optional<QString> chatModel() const;
    std::optional<int> topN() const;
    QString chatMode() const;
    std::optional<QString> pfpFilename() const;
    std::optional<QString> agentProvider() const;
    std::optional<QString> agentModel() const;
    std::optional<QString> queryRefusalResponse() const;
    QString vectorSearchMode() const;
    std::optional<bool> compressEnabled() const;
    std::optional<double> compressThreshold() const;
    std::optional<int> compressContextLen() const;

private:
    static std::optional<QString> optionalString(const QJsonObject &obj, const char *key);
    static std::optional<double> optionalDouble(const QJsonObject &obj, const char *key);
    static std::optional<int> optionalInt(const QJsonObject &obj, const char *key);
    static std::optional<bool> optionalBool(const QJsonObject &obj, const char *key);

    int m_id = 0;
    QString m_name;
    QString m_slug;
    std::optional<QString> m_vectorTag;
    QDateTime m_createdAt;
    QDateTime m_lastUpdatedAt;
    int m_openAiHistory = 20;
    std::optional<double> m_openAiTemp;
    std::optional<QString> m_openAiPrompt;
    std::optional<double> m_similarityThreshold;
    std::optional<QString> m_chatProvider;
    std::optional<QString> m_chatModel;
    std::optional<int> m_topN;
    QString m_chatMode = QStringLiteral("chat");
    std::optional<QString> m_pfpFilename;
    std::optional<QString> m_agentProvider;
    std::optional<QString> m_agentModel;
    std::optional<QString> m_queryRefusalResponse;
    QString m_vectorSearchMode = QStringLiteral("default");
    std::optional<bool> m_compressEnabled;
    std::optional<double> m_compressThreshold;
    std::optional<int> m_compressContextLen;
};

#endif // HERMIND_WORKSPACE_H
