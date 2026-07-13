#ifndef AGENT_CONFIG_STATE_H
#define AGENT_CONFIG_STATE_H

#include <QString>
#include <QJsonObject>

class AgentConfigState
{
public:
    AgentConfigState() = default;

    void setOriginalProvider(const QString &provider);
    void setOriginalModel(const QString &model);

    void setProvider(const QString &provider);
    void setModel(const QString &model);

    QString provider() const;
    QString model() const;
    bool isDirty() const;
    bool isPerformanceWarning() const;
    bool isModelSupported(const QString &provider, const QString &model) const;

    QJsonObject buildUpdatePayload() const;

private:
    QString m_originalProvider = QStringLiteral("default");
    QString m_originalModel;
    QString m_provider = QStringLiteral("default");
    QString m_model;
};

#endif // AGENT_CONFIG_STATE_H
