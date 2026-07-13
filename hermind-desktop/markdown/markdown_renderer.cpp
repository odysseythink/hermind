#include "markdown_renderer.h"

#include "html_generator.h"
#include "html_sanitizer.h"
#include "formula_renderer.h"
#include "qt_builtin_parser.h"
#include "syntax_highlighter.h"

#include <QDebug>
#include <QLabel>
#include <QTextBrowser>
#include <QTextDocument>

MarkdownRenderer::MarkdownRenderer(QObject *parent)
    : QObject(parent)
    , m_parser(std::make_unique<QtBuiltinParser>())
    // Deliberate degradation (M11): QTextBrowser cannot run JavaScript, so
    // full syntax highlighting / KaTeX typesetting is impossible without a
    // JS-capable view. The Null* implementations escape and wrap content so
    // code and latex remain readable; swap in real implementations here if a
    // QWebEngine-based view is ever adopted.
    , m_highlighter(std::make_unique<NullSyntaxHighlighter>())
    , m_formulaRenderer(std::make_unique<NullFormulaRenderer>())
{
}

MarkdownRenderer::~MarkdownRenderer()
{
    if (m_currentWidget)
        delete m_currentWidget;
}

void MarkdownRenderer::setMarkdown(const QString &markdown, bool darkMode)
{
    m_markdown = markdown;
    m_darkMode = darkMode;
    renderInternal();
}

void MarkdownRenderer::setDarkMode(bool darkMode)
{
    if (m_darkMode == darkMode)
        return;
    m_darkMode = darkMode;
    renderInternal();
}

QWidget *MarkdownRenderer::widget() const
{
    return m_currentWidget;
}

void MarkdownRenderer::renderInternal()
{
    if (m_currentWidget) {
        m_currentWidget->deleteLater();
        m_currentWidget = nullptr;
    }

    if (m_markdown.isEmpty()) {
        fallbackToPlainText(QString());
        return;
    }

    QString error;
    std::unique_ptr<MarkdownDocument> doc = m_parser->parse(m_markdown, &error);
    if (!doc || doc->isEmpty()) {
        const QString reason = error.isEmpty()
            ? QStringLiteral("parser returned an empty document") : error;
        qWarning() << "MarkdownRenderer: parse failed:" << reason;
        fallbackToPlainText(m_markdown);
        emit renderFailed(reason);
        return;
    }

    HtmlGenerationOptions options;
    options.darkMode = m_darkMode;
    options.enableRawHtml = false;
    options.highlighter = m_highlighter.get();
    options.formulaRenderer = m_formulaRenderer.get();
    const QString rawHtml = HtmlGenerator::generate(*doc, options);
    if (rawHtml.isEmpty()) {
        const QString reason = QStringLiteral("HTML generation produced empty output");
        qWarning() << "MarkdownRenderer:" << reason;
        fallbackToPlainText(m_markdown);
        emit renderFailed(reason);
        return;
    }

    // HtmlSanitizer wraps its input in a synthetic root element; a DOCTYPE
    // inside that root is a fatal parse error that would truncate all output.
    const int idx = rawHtml.indexOf(QStringLiteral("<html"));
    const QString safeHtml = HtmlSanitizer::sanitize(idx >= 0 ? rawHtml.mid(idx) : rawHtml);

    auto *browser = new QTextBrowser();
    browser->setOpenLinks(false);
    browser->setOpenExternalLinks(false);
    browser->setFrameShape(QFrame::NoFrame);
    browser->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    browser->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    browser->setHtml(safeHtml);

    browser->document()->adjustSize();
    const int h = static_cast<int>(browser->document()->size().height());
    if (h > 0) {
        browser->setMinimumHeight(h + 4);
        browser->setMaximumHeight(h + 4);
    }

    connect(browser, &QTextBrowser::anchorClicked,
            this, &MarkdownRenderer::linkActivated);

    m_currentWidget = browser;
}

void MarkdownRenderer::fallbackToPlainText(const QString &text)
{
    auto *label = new QLabel(text);
    label->setWordWrap(true);
    label->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_currentWidget = label;
}
