#include "tst_agent_config_state.h"

#include <QtTest>
#include "agent_config_state.h"

void TestAgentConfigState::defaultProviderIsNotDirty()
{
    AgentConfigState state;
    state.setOriginalProvider(QStringLiteral("default"));
    QVERIFY(!state.isDirty());
}

void TestAgentConfigState::selectingProviderMakesDirty()
{
    AgentConfigState state;
    state.setOriginalProvider(QStringLiteral("default"));
    state.setProvider(QStringLiteral("openai"));
    QVERIFY(state.isDirty());
}

void TestAgentConfigState::localProviderShowsPerformanceWarning()
{
    AgentConfigState state;
    state.setProvider(QStringLiteral("ollama"));
    QVERIFY(state.isPerformanceWarning());
    state.setProvider(QStringLiteral("openai"));
    QVERIFY(!state.isPerformanceWarning());
}

void TestAgentConfigState::unsupportedOpenAiModelsAreRejected()
{
    AgentConfigState state;
    QVERIFY(state.isModelSupported(QStringLiteral("openai"), QStringLiteral("gpt-4o")));
    QVERIFY(!state.isModelSupported(QStringLiteral("openai"), QStringLiteral("gpt-4-turbo")));
    QVERIFY(!state.isModelSupported(QStringLiteral("openai"), QStringLiteral("o1-preview")));
    QVERIFY(state.isModelSupported(QStringLiteral("anthropic"), QStringLiteral("claude-3")));
}

void TestAgentConfigState::payloadForDefaultProviderEmptiesAgentProvider()
{
    AgentConfigState state;
    state.setOriginalProvider(QStringLiteral("default"));
    state.setProvider(QStringLiteral("default"));
    const QJsonObject payload = state.buildUpdatePayload();
    QCOMPARE(payload.value(QStringLiteral("agentProvider")).toString(), QString());
    QCOMPARE(payload.value(QStringLiteral("agentModel")).toString(), QString());
}

void TestAgentConfigState::payloadForSelectedProvider()
{
    AgentConfigState state;
    state.setOriginalProvider(QStringLiteral("default"));
    state.setProvider(QStringLiteral("openai"));
    state.setModel(QStringLiteral("gpt-4o-mini"));
    const QJsonObject payload = state.buildUpdatePayload();
    QCOMPARE(payload.value(QStringLiteral("agentProvider")).toString(), QStringLiteral("openai"));
    QCOMPARE(payload.value(QStringLiteral("agentModel")).toString(), QStringLiteral("gpt-4o-mini"));
}
