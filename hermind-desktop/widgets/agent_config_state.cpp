#include "agent_config_state.h"

static const QStringList &localProviders()
{
    static const QStringList list = {
        QStringLiteral("lmstudio"),
        QStringLiteral("ollama"),
        QStringLiteral("localai"),
        QStringLiteral("koboldcpp"),
        QStringLiteral("textgenwebui"),
        QStringLiteral("docker-model-runner"),
    };
    return list;
}

static const QStringList &unsupportedOpenAiModels()
{
    static const QStringList list = {
        QStringLiteral("gpt-3.5-turbo-0301"),
        QStringLiteral("gpt-4-turbo-2024-04-09"),
        QStringLiteral("gpt-4-turbo"),
        QStringLiteral("o1-preview"),
        QStringLiteral("o1-preview-2024-09-12"),
        QStringLiteral("o1-mini"),
        QStringLiteral("o1-mini-2024-09-12"),
        QStringLiteral("o3-mini"),
        QStringLiteral("o3-mini-2025-01-31"),
    };
    return list;
}

void AgentConfigState::setOriginalProvider(const QString &provider)
{
    m_originalProvider = provider.isEmpty() ? QStringLiteral("default") : provider;
    if (m_provider == QStringLiteral("default"))
        m_provider = m_originalProvider;
}

void AgentConfigState::setOriginalModel(const QString &model)
{
    m_originalModel = model;
    if (m_model.isEmpty())
        m_model = model;
}

void AgentConfigState::setProvider(const QString &provider)
{
    m_provider = provider.isEmpty() ? QStringLiteral("default") : provider;
    if (m_provider == QStringLiteral("default"))
        m_model.clear();
}

void AgentConfigState::setModel(const QString &model)
{
    m_model = model;
}

QString AgentConfigState::provider() const
{
    return m_provider;
}

QString AgentConfigState::model() const
{
    return m_model;
}

bool AgentConfigState::isDirty() const
{
    return m_provider != m_originalProvider || m_model != m_originalModel;
}

bool AgentConfigState::isPerformanceWarning() const
{
    return localProviders().contains(m_provider);
}

bool AgentConfigState::isModelSupported(const QString &provider, const QString &model) const
{
    if (provider == QStringLiteral("openai") && unsupportedOpenAiModels().contains(model))
        return false;
    return true;
}

QJsonObject AgentConfigState::buildUpdatePayload() const
{
    QJsonObject obj;
    obj.insert(QStringLiteral("agentProvider"),
               m_provider == QStringLiteral("default") ? QString() : m_provider);
    obj.insert(QStringLiteral("agentModel"), m_model);
    return obj;
}
