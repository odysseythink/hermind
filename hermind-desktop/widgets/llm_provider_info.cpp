#include "llm_provider_info.h"

static const QVector<LlmProviderInfo> &providerList()
{
    static const QVector<LlmProviderInfo> list = {
        { QStringLiteral("default"), QStringLiteral("System default"),
          QStringLiteral("Use the system LLM preference for this workspace."), false },
        { QStringLiteral("openai"), QStringLiteral("OpenAI"),
          QStringLiteral("OpenAI API (GPT-4, GPT-3.5, etc.)."), true },
        { QStringLiteral("anthropic"), QStringLiteral("Anthropic"),
          QStringLiteral("Claude models via Anthropic API."), true },
        { QStringLiteral("ollama"), QStringLiteral("Ollama"),
          QStringLiteral("Local models via Ollama."), true },
        { QStringLiteral("lmstudio"), QStringLiteral("LM Studio"),
          QStringLiteral("Local server via LM Studio."), true },
        { QStringLiteral("localai"), QStringLiteral("LocalAI"),
          QStringLiteral("OpenAI-compatible local server."), true },
        { QStringLiteral("azure"), QStringLiteral("Azure OpenAI"),
          QStringLiteral("Enter model name manually."), false },
        { QStringLiteral("gemini"), QStringLiteral("Gemini"),
          QStringLiteral("Google Gemini API."), true },
        { QStringLiteral("mistral"), QStringLiteral("Mistral"),
          QStringLiteral("Mistral API."), true },
        { QStringLiteral("deepseek"), QStringLiteral("DeepSeek"),
          QStringLiteral("DeepSeek API."), true },
    };
    return list;
}

const QVector<LlmProviderInfo> &LlmProviderInfo::all() { return providerList(); }

const LlmProviderInfo *LlmProviderInfo::byId(const QString &id)
{
    for (const auto &p : providerList()) {
        if (p.id == id)
            return &p;
    }
    return nullptr;
}

QStringList LlmProviderInfo::ids()
{
    QStringList out;
    for (const auto &p : providerList())
        out.append(p.id);
    return out;
}
