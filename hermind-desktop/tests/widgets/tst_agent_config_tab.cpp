#include "tst_agent_config_tab.h"

#include <QComboBox>
#include <QLabel>
#include <QSignalSpy>
#include <QTest>
#include <QJsonObject>

#include "agent_config_tab.h"
#include "hermind_workspace.h"

static HermindWorkspace workspaceWithAgent(const QString &provider,
                                           const QString &model)
{
    QJsonObject obj;
    obj.insert(QStringLiteral("id"), 1);
    obj.insert(QStringLiteral("name"), QStringLiteral("Test Workspace"));
    obj.insert(QStringLiteral("slug"), QStringLiteral("test-workspace"));
    obj.insert(QStringLiteral("createdAt"), QStringLiteral("2026-01-01T00:00:00Z"));
    obj.insert(QStringLiteral("lastUpdatedAt"), QStringLiteral("2026-01-01T00:00:00Z"));
    obj.insert(QStringLiteral("agentProvider"), provider);
    obj.insert(QStringLiteral("agentModel"), model);
    return HermindWorkspace::fromJson(obj);
}

void TestAgentConfigTab::loadWorkspaceSetsProviderAndModel()
{
    AgentConfigTab tab;
    tab.loadFromWorkspace(workspaceWithAgent(QStringLiteral("openai"),
                                             QStringLiteral("gpt-4o-mini")));

    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("agentProviderCombo"));
    auto *modelCombo = tab.findChild<QComboBox *>(QStringLiteral("agentModelCombo"));
    QVERIFY(providerCombo);
    QVERIFY(modelCombo);

    QCOMPARE(providerCombo->currentData().toString(), QStringLiteral("openai"));
    QCOMPARE(modelCombo->currentText(), QStringLiteral("gpt-4o-mini"));
    QVERIFY(!tab.isDirty());
}

void TestAgentConfigTab::changingProviderEmitsDirty()
{
    AgentConfigTab tab;
    tab.loadFromWorkspace(workspaceWithAgent(QStringLiteral("default"), QString()));

    QSignalSpy spy(&tab, &AgentConfigTab::dirtyChanged);
    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("agentProviderCombo"));
    QVERIFY(providerCombo);

    const int openAiIndex = providerCombo->findData(QStringLiteral("openai"));
    QVERIFY(openAiIndex >= 0);
    providerCombo->setCurrentIndex(openAiIndex);

    QVERIFY(tab.isDirty());
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.takeFirst().at(0).toBool(), true);
}

void TestAgentConfigTab::localProviderShowsPerformanceWarning()
{
    AgentConfigTab tab;
    tab.show();
    tab.loadFromWorkspace(workspaceWithAgent(QStringLiteral("default"), QString()));

    QSignalSpy spy(&tab, &AgentConfigTab::performanceWarningChanged);
    auto *providerCombo = tab.findChild<QComboBox *>(QStringLiteral("agentProviderCombo"));
    QVERIFY(providerCombo);

    const int ollamaIndex = providerCombo->findData(QStringLiteral("ollama"));
    QVERIFY(ollamaIndex >= 0);
    providerCombo->setCurrentIndex(ollamaIndex);

    auto *warningLabel = tab.findChild<QLabel *>(QStringLiteral("agentPerformanceWarning"));
    QVERIFY(warningLabel);
    QVERIFY(warningLabel->isVisible());

    QCOMPARE(spy.count(), 1);
    QVERIFY(spy.takeFirst().at(0).toBool());
}

void TestAgentConfigTab::unsupportedOpenAiModelShowsWarning()
{
    AgentConfigTab tab;
    tab.show();
    tab.loadFromWorkspace(workspaceWithAgent(QStringLiteral("openai"),
                                             QStringLiteral("gpt-4o-mini")));

    QSignalSpy spy(&tab, &AgentConfigTab::performanceWarningChanged);
    auto *modelCombo = tab.findChild<QComboBox *>(QStringLiteral("agentModelCombo"));
    QVERIFY(modelCombo);

    modelCombo->setCurrentText(QStringLiteral("o1-preview"));

    auto *warningLabel = tab.findChild<QLabel *>(QStringLiteral("agentPerformanceWarning"));
    QVERIFY(warningLabel);
    QVERIFY(warningLabel->isVisible());

    QCOMPARE(spy.count(), 1);
    QVERIFY(spy.takeFirst().at(0).toBool());
}

void TestAgentConfigTab::buildUpdatePayloadContainsAgentFields()
{
    AgentConfigTab tab;
    tab.loadFromWorkspace(workspaceWithAgent(QStringLiteral("anthropic"),
                                             QStringLiteral("claude-3-5-sonnet")));

    const QJsonObject payload = tab.buildUpdatePayload();
    QCOMPARE(payload.value(QStringLiteral("agentProvider")).toString(),
             QStringLiteral("anthropic"));
    QCOMPARE(payload.value(QStringLiteral("agentModel")).toString(),
             QStringLiteral("claude-3-5-sonnet"));
}
