#include "qt_builtin_parser.h"

#include <QTextDocument>

#include <utility>

QtBuiltinDocument::QtBuiltinDocument(QString html)
    : m_html(std::move(html))
{
}

QString QtBuiltinDocument::toHtml(const HtmlGenerationOptions &) const
{
    return m_html;
}

bool QtBuiltinDocument::isEmpty() const
{
    return m_html.isEmpty();
}

std::unique_ptr<MarkdownDocument> QtBuiltinParser::parse(const QString &markdown, QString *error)
{
    if (error)
        error->clear();

    // QTextDocument::setMarkdown("") still emits a full <html>...<body></body>
    // skeleton, so treat trimmed-empty input as an empty document explicitly.
    if (markdown.trimmed().isEmpty())
        return std::make_unique<QtBuiltinDocument>(QString());

    QTextDocument doc;
    doc.setMarkdown(markdown);
    return std::make_unique<QtBuiltinDocument>(doc.toHtml());
}

bool QtBuiltinParser::supportsGfm() const
{
    return true;
}

bool QtBuiltinParser::supportsMath() const
{
    return false;
}
