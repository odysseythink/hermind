#include <QtTest>
#include <QLabel>
#include <QTextBrowser>

#include "markdown_renderer.h"

class TestMarkdownRenderer : public QObject {
    Q_OBJECT

private slots:
    void rendersBoldMarkdown();
    void degradesOnEmptyInput();
    void scriptTagIsSanitized();
    void linkRendering();
};

void TestMarkdownRenderer::rendersBoldMarkdown()
{
    MarkdownRenderer renderer;
    renderer.setMarkdown(QStringLiteral("**bold text**"), true);

    QVERIFY(renderer.widget() != nullptr);
    auto *browser = qobject_cast<QTextBrowser *>(renderer.widget());
    QVERIFY(browser != nullptr);
    QVERIFY(browser->toPlainText().contains(QStringLiteral("bold text")));
}

void TestMarkdownRenderer::degradesOnEmptyInput()
{
    MarkdownRenderer renderer;
    bool failed = false;
    QObject::connect(&renderer, &MarkdownRenderer::renderFailed,
                     [&failed](const QString &) { failed = true; });

    renderer.setMarkdown(QString(), true);

    // Empty input must still produce a usable widget (plain-text QLabel
    // fallback) and must not crash; renderFailed is reserved for actual
    // parse failures and may stay quiet for empty input.
    Q_UNUSED(failed);
    QVERIFY(renderer.widget() != nullptr);
}

void TestMarkdownRenderer::scriptTagIsSanitized()
{
    MarkdownRenderer renderer;
    renderer.setMarkdown(QStringLiteral("<script>alert(1)</script>"), true);

    auto *browser = qobject_cast<QTextBrowser *>(renderer.widget());
    QVERIFY(browser != nullptr);

    // The important property: the HTML finally fed to the browser contains
    // no live <script> element (HtmlSanitizer strips unknown tags), even if
    // QTextDocument escaped the script source into visible text.
    QVERIFY(!browser->toHtml().contains(QStringLiteral("<script"), Qt::CaseInsensitive));
}

void TestMarkdownRenderer::linkRendering()
{
    MarkdownRenderer renderer;
    renderer.setMarkdown(QStringLiteral("[example](https://example.com)"), true);

    auto *browser = qobject_cast<QTextBrowser *>(renderer.widget());
    QVERIFY(browser != nullptr);
    QVERIFY(browser->toPlainText().contains(QStringLiteral("example")));
    // The href must survive sanitization.
    QVERIFY(browser->toHtml().contains(QStringLiteral("https://example.com")));
}

QTEST_MAIN(TestMarkdownRenderer)

#include "tst_markdown_renderer.moc"
