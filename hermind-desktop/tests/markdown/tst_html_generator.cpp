#include <QtTest>

#include "html_generator.h"
#include "html_sanitizer.h"
#include "qt_builtin_parser.h"

class TestHtmlGenerator : public QObject {
    Q_OBJECT

private slots:
    void loadThemeCssFromResources();
    void generateProducesFullPage();
    void generatedPageSurvivesSanitizer();
    void codeBlockPostProcessing();
    void emptyMarkdownIsEmpty();
};

void TestHtmlGenerator::loadThemeCssFromResources()
{
    const QString dark = HtmlGenerator::loadThemeCss(true);
    const QString light = HtmlGenerator::loadThemeCss(false);
    QVERIFY(dark.contains(QStringLiteral("background-color")));
    QVERIFY(light.contains(QStringLiteral("background-color")));
    QVERIFY(dark != light);
}

void TestHtmlGenerator::generateProducesFullPage()
{
    QtBuiltinParser parser;
    auto doc = parser.parse(QStringLiteral("**bold**"));
    QVERIFY(doc);
    QVERIFY(!doc->isEmpty());

    const QString page = HtmlGenerator::generate(*doc, HtmlGenerationOptions());
    QVERIFY(page.startsWith(QStringLiteral("<!DOCTYPE html")));
    QVERIFY(page.contains(QStringLiteral("<meta charset=\"utf-8\"/>")));
    QVERIFY(page.contains(QStringLiteral("<style>")));
    QVERIFY(page.contains(QStringLiteral("bold")));
}

void TestHtmlGenerator::generatedPageSurvivesSanitizer()
{
    QtBuiltinParser parser;
    auto doc = parser.parse(QStringLiteral("# Hi\n\n**bold**"));
    QVERIFY(doc);

    const QString page = HtmlGenerator::generate(*doc, HtmlGenerationOptions());

    // The sanitizer wraps its input in a synthetic root element, and Qt's
    // QXmlStreamReader rejects a DOCTYPE inside element content ("Unexpected
    // token type DTD in Body") and stops there. Consumers therefore sanitize
    // the document from the <html> element onward; this test verifies that
    // everything after the DOCTYPE is well-formed XML that survives intact.
    const int htmlIdx = page.indexOf(QStringLiteral("<html"));
    QVERIFY(htmlIdx >= 0);
    const QString sanitized = HtmlSanitizer::sanitize(page.mid(htmlIdx));
    QVERIFY(sanitized.contains(QStringLiteral("Hi")));
    QVERIFY(sanitized.contains(QStringLiteral("bold")));
    // Closing tag proves the whole document parsed without truncation.
    QVERIFY(sanitized.contains(QStringLiteral("</html>")));
}

void TestHtmlGenerator::codeBlockPostProcessing()
{
    QtBuiltinParser parser;
    auto doc = parser.parse(QStringLiteral("```cpp\nint main() { return 0; }\n```\n"));
    QVERIFY(doc);

    const QString page = HtmlGenerator::generate(*doc, HtmlGenerationOptions());
    QVERIFY(page.contains(QStringLiteral("code-block")));
    QVERIFY(page.contains(QStringLiteral("code-header")));
    QVERIFY(page.contains(QStringLiteral("copy-btn")));
    QVERIFY(page.contains(QStringLiteral("int main()")));
}

void TestHtmlGenerator::emptyMarkdownIsEmpty()
{
    QtBuiltinParser parser;
    auto doc = parser.parse(QStringLiteral("  \n  "));
    QVERIFY(doc);
    QVERIFY(doc->isEmpty());
}

QTEST_GUILESS_MAIN(TestHtmlGenerator)

#include "tst_html_generator.moc"
