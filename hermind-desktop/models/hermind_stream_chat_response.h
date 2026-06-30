#ifndef HERMIND_STREAM_CHAT_RESPONSE_H
#define HERMIND_STREAM_CHAT_RESPONSE_H

#include <QString>
#include <QJsonObject>
#include <QJsonArray>
#include <optional>

class HermindStreamChatResponse
{
public:
    HermindStreamChatResponse() = default;

    static HermindStreamChatResponse fromJson(const QJsonObject &obj);
    QJsonObject toJson() const;

    QString uuid() const;
    QString type() const;
    std::optional<QString> textResponse() const;
    QJsonArray sources() const;
    bool close() const;
    std::optional<QString> error() const;
    bool animate() const;
    std::optional<int> chatId() const;
    std::optional<QString> action() const;
    std::optional<QString> websocketUUID() const;
    std::optional<QString> websocketToken() const;
    std::optional<QString> routedTo() const;
    QJsonObject metrics() const;
    QJsonObject raw() const;

private:
    static std::optional<QString> optionalString(const QJsonObject &obj, const char *key);
    static std::optional<int> optionalInt(const QJsonObject &obj, const char *key);

    QJsonObject m_raw;
    QString m_uuid;
    QString m_type;
    std::optional<QString> m_textResponse;
    QJsonArray m_sources;
    bool m_close = false;
    std::optional<QString> m_error;
    bool m_animate = false;
    std::optional<int> m_chatId;
    std::optional<QString> m_action;
    std::optional<QString> m_websocketUUID;
    std::optional<QString> m_websocketToken;
    std::optional<QString> m_routedTo;
    QJsonObject m_metrics;
};

#endif // HERMIND_STREAM_CHAT_RESPONSE_H
