#include <QtTest>

#include "formula_renderer.h"
#include "html_generator.h"

namespace {

// Minimal MarkdownDocument stub so we can exercise HtmlGenerator without
// pulling in a real parser (keeps this test app-less).
class StubDocument : public MarkdownDocument {
public:
    QString toHtml(const HtmlGenerationOptions &) const override
    {
        return QStringLiteral("<p>body</p>");
    }
    bool isEmpty() const override { return false; }
};

} // namespace

class TestFormulaRenderer : public QObject {
    Q_OBJECT

private slots:
    void renderInline_formulaHtml();
    void renderBlock_formulaHtml();
    void requiredCss_notEmpty();
    void emptyLatex_returnsEmpty();
    void katexRenderer_wrapsInKatexClass();
    void katexRequiredCss_loadsFromResource();
    void escapesLatexSpecialChars();
    void generator_appendsFormulaCss();
};

void TestFormulaRenderer::renderInline_formulaHtml()
{
    NullFormulaRenderer r;
    const QString out = r.render(QStringLiteral("E=mc^2"), false);
    QVERIFY(out.contains(QStringLiteral("E=mc^2")));
    QVERIFY(out.contains(QStringLiteral("<code")));
    QVERIFY(out.contains(QStringLiteral("math-inline")));
}

void TestFormulaRenderer::renderBlock_formulaHtml()
{
    NullFormulaRenderer r;
    const QString latex = QStringLiteral("\\sum_{i=1}^{n} x_i");
    const QString out = r.render(latex, true);
    QVERIFY(out.contains(latex));
    QVERIFY(out.contains(QStringLiteral("<div")));
    QVERIFY(out.contains(QStringLiteral("math-block")));
}

void TestFormulaRenderer::requiredCss_notEmpty()
{
    NullFormulaRenderer r;
    QVERIFY(!r.requiredCss().isEmpty());
}

void TestFormulaRenderer::emptyLatex_returnsEmpty()
{
    NullFormulaRenderer r;
    QVERIFY(r.render(QString(), false).isEmpty());
    QVERIFY(r.render(QString(), true).isEmpty());
}

void TestFormulaRenderer::katexRenderer_wrapsInKatexClass()
{
    KatexFormulaRenderer r;
    const QString inlineOut = r.render(QStringLiteral("x^2"), false);
    QVERIFY(inlineOut.contains(QStringLiteral("katex")));
    QVERIFY(inlineOut.contains(QStringLiteral("x^2")));

    const QString displayOut = r.render(QStringLiteral("x^2"), true);
    QVERIFY(displayOut.contains(QStringLiteral("katex-display")));
    QVERIFY(displayOut.contains(QStringLiteral("katex")));
    QVERIFY(displayOut.contains(QStringLiteral("x^2")));
}

void TestFormulaRenderer::katexRequiredCss_loadsFromResource()
{
    KatexFormulaRenderer r;
    const QString css = r.requiredCss();
    if (css.isEmpty())
        QSKIP(":/katex/katex.min.css not available in test resources");
    QVERIFY2(css.contains(QStringLiteral(".katex")),
             qPrintable(QStringLiteral("katex CSS missing .katex rules, got: %1").arg(css.left(120))));
}

void TestFormulaRenderer::escapesLatexSpecialChars()
{
    NullFormulaRenderer r;
    const QString out = r.render(QStringLiteral("<b>not bold</b>"), false);
    QVERIFY(!out.contains(QStringLiteral("<b>")));
    QVERIFY(out.contains(QStringLiteral("&lt;b&gt;")));

    KatexFormulaRenderer kr;
    const QString kOut = kr.render(QStringLiteral("<b>not bold</b>"), false);
    QVERIFY(!kOut.contains(QStringLiteral("<b>")));
    QVERIFY(kOut.contains(QStringLiteral("&lt;b&gt;")));
}

void TestFormulaRenderer::generator_appendsFormulaCss()
{
    StubDocument doc;
    NullFormulaRenderer r;
    HtmlGenerationOptions options;
    options.formulaRenderer = &r;

    const QString page = HtmlGenerator::generate(doc, options);
    QVERIFY(page.contains(QStringLiteral(".math-inline")));
    QVERIFY(page.contains(QStringLiteral(".math-block")));

    // Without a renderer no formula CSS should be injected.
    HtmlGenerationOptions noRenderer;
    const QString plain = HtmlGenerator::generate(doc, noRenderer);
    QVERIFY(!plain.contains(QStringLiteral(".math-inline")));
}

QTEST_APPLESS_MAIN(TestFormulaRenderer)

#include "tst_formula_renderer.moc"
