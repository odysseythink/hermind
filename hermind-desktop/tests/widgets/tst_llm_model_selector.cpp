#include "tst_llm_model_selector.h"
#include <QtTest>
#include "llm_model_selector.h"

void TestLlmModelSelector::defaultProviderReturnsEmptyModels()
{
    const QStringList models = LlmModelSelector::modelsForProvider(
        QStringLiteral("default"), QStringList() << QStringLiteral("x"));
    QVERIFY(models.isEmpty());
    QVERIFY(!LlmModelSelector::supportsModelSelection(QStringLiteral("default")));
}

void TestLlmModelSelector::azureRequiresManualInput()
{
    QVERIFY(!LlmModelSelector::supportsModelSelection(QStringLiteral("azure")));
    QVERIFY(LlmModelSelector::isManualModelInput(QStringLiteral("azure")));
    const QStringList models = LlmModelSelector::modelsForProvider(
        QStringLiteral("azure"), QStringList() << QStringLiteral("gpt-4"));
    QVERIFY(models.isEmpty());
}

void TestLlmModelSelector::openAiUsesDiscoveredModels()
{
    const QStringList discovered = { QStringLiteral("gpt-4o"), QStringLiteral("gpt-4o-mini") };
    const QStringList models = LlmModelSelector::modelsForProvider(
        QStringLiteral("openai"), discovered);
    QCOMPARE(models, discovered);
}

void TestLlmModelSelector::cohereFallsBackToDefaults()
{
    const QStringList models = LlmModelSelector::modelsForProvider(
        QStringLiteral("cohere"), QStringList());
    QVERIFY(models.contains(QStringLiteral("command-r")));
    QVERIFY(models.contains(QStringLiteral("command")));
}

void TestLlmModelSelector::duplicatesAreRemoved()
{
    const QStringList discovered = { QStringLiteral("command-r"), QStringLiteral("custom") };
    const QStringList models = LlmModelSelector::modelsForProvider(
        QStringLiteral("cohere"), discovered);
    QCOMPARE(models.count(QStringLiteral("command-r")), 1);
    QVERIFY(models.contains(QStringLiteral("custom")));
    // defaults come first
    QCOMPARE(models.indexOf(QStringLiteral("command-r")), 0);
}

void TestLlmModelSelector::recommendedTemperature()
{
    QCOMPARE(LlmModelSelector::recommendedTemperature(QStringLiteral("mistral")), 0.0);
    QCOMPARE(LlmModelSelector::recommendedTemperature(QStringLiteral("openai")), 0.7);
    QCOMPARE(LlmModelSelector::recommendedTemperature(QStringLiteral("unknown")), 0.7);
}
