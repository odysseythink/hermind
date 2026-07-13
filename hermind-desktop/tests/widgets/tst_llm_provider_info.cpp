#include "tst_llm_provider_info.h"
#include <QtTest>
#include "llm_provider_info.h"

void TestLlmProviderInfo::cohereHasDefaultModels()
{
    const LlmProviderInfo *p = LlmProviderInfo::byId("cohere");
    QVERIFY(p);
    QVERIFY(p->defaultModels.contains(QStringLiteral("command-r")));
    QVERIFY(p->defaultModels.contains(QStringLiteral("command")));
}

void TestLlmProviderInfo::openAiDefaultModelsEmpty()
{
    const LlmProviderInfo *p = LlmProviderInfo::byId("openai");
    QVERIFY(p);
    QVERIFY(p->defaultModels.isEmpty());
}

void TestLlmProviderInfo::defaultProviderHasNoModels()
{
    const LlmProviderInfo *p = LlmProviderInfo::byId("default");
    QVERIFY(p);
    QVERIFY(!p->supportsModelSelection);
    QVERIFY(p->defaultModels.isEmpty());
}
