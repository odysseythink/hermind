#include <QtTest>

#include "syntax_highlighter.h"
#include "html_generator.h"
#include "html_sanitizer.h"
#include "qt_builtin_parser.h"

class TestSyntaxHighlighter : public QObject {
    Q_OBJECT

private slots:
    void knownLanguage_returnsHighlightedHtml();
    void unknownLanguage_returnsEscapedPlainText();
    void textLanguage_omitsLanguageClass();
    void emptyCode_returnsEmpty();
    void escapesHtmlInCode();
    void supportedLanguages_isEmpty();
    void generator_usesHighlighter();
};

void TestSyntaxHighlighter::knownLanguage_returnsHighlightedHtml()
{
    NullSyntaxHighlighter hl;
    const QString out = hl.highlight(QStringLiteral("int main() {}"),
                                     QStringLiteral("cpp"), true);
    QVERIFY(out.contains(QStringLiteral("int main()")));
    QVERIFY(out.contains(QStringLiteral("<pre")));
    QVERIFY(out.contains(QStringLiteral("<code")));
    QVERIFY(out.contains(QStringLiteral("language-cpp")));
}

void TestSyntaxHighlighter::unknownLanguage_returnsEscapedPlainText()
{
    NullSyntaxHighlighter hl;
    const QString out = hl.highlight(QStringLiteral("hello"),
                                     QStringLiteral("foolang"), true);
    QVERIFY(out.contains(QStringLiteral("<pre")));
    QVERIFY(out.contains(QStringLiteral("<code")));
    QVERIFY(out.contains(QStringLiteral("hello")));
}

void TestSyntaxHighlighter::textLanguage_omitsLanguageClass()
{
    NullSyntaxHighlighter hl;
    const QString out = hl.highlight(QStringLiteral("plain"),
                                     QStringLiteral("text"), true);
    QVERIFY(out.contains(QStringLiteral("plain")));
    QVERIFY(!out.contains(QStringLiteral("language-")));
}

void TestSyntaxHighlighter::emptyCode_returnsEmpty()
{
    NullSyntaxHighlighter hl;
    QVERIFY(hl.highlight(QString(), QStringLiteral("cpp"), true).isEmpty());
}

void TestSyntaxHighlighter::escapesHtmlInCode()
{
    NullSyntaxHighlighter hl;
    const QString out = hl.highlight(QStringLiteral("<script>alert(1)</script>"),
                                     QStringLiteral("js"), true);
    QVERIFY(!out.contains(QStringLiteral("<script>")));
    QVERIFY(out.contains(QStringLiteral("&lt;script&gt;")));
}

void TestSyntaxHighlighter::supportedLanguages_isEmpty()
{
    NullSyntaxHighlighter hl;
    QVERIFY(hl.supportedLanguages().isEmpty());
}

void TestSyntaxHighlighter::generator_usesHighlighter()
{
    QtBuiltinParser parser;
    auto doc = parser.parse(QStringLiteral("```cpp\nint main() { return 0; }\n```\n"));
    QVERIFY(doc);
    QVERIFY(!doc->isEmpty());

    NullSyntaxHighlighter hl;
    HtmlGenerationOptions options;
    options.highlighter = &hl;

    const QString page = HtmlGenerator::generate(*doc, options);
    QVERIFY(page.contains(QStringLiteral("language-cpp")));
    QVERIFY(page.contains(QStringLiteral("code-block")));
    QVERIFY(page.contains(QStringLiteral("copy-btn")));
    QVERIFY(page.contains(QStringLiteral("int main()")));

    // Must survive the sanitizer without truncation (strip the DOCTYPE first,
    // same as MarkdownRenderer does: the sanitizer's synthetic root rejects
    // a DOCTYPE in element content).
    const int idx = page.indexOf(QStringLiteral("<html"));
    QVERIFY(idx >= 0);
    const QString sanitized = HtmlSanitizer::sanitize(page.mid(idx));
    QVERIFY(sanitized.contains(QStringLiteral("language-cpp")));
    QVERIFY(sanitized.contains(QStringLiteral("code-block")));
    QVERIFY(sanitized.contains(QStringLiteral("int main()")));
    QVERIFY(sanitized.contains(QStringLiteral("</html>")));
}

QTEST_GUILESS_MAIN(TestSyntaxHighlighter)

#include "tst_syntax_highlighter.moc"
