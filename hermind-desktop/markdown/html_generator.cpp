#include "html_generator.h"

#include <QFile>
#include <QRegularExpression>

namespace {

// QTextDocument::toHtml() emits a full document; pull out just the body
// content so we don't nest documents inside our generated page. Falls back
// to the whole string if no <body> element is found.
QString extractBody(const QString &html)
{
    static const QRegularExpression bodyRe(QStringLiteral("<body[^>]*>(.*)</body>"),
                                           QRegularExpression::DotMatchesEverythingOption
                                               | QRegularExpression::CaseInsensitiveOption);
    const QRegularExpressionMatch m = bodyRe.match(html);
    if (m.hasMatch())
        return m.captured(1);
    return html;
}

QString extractLanguage(const QString &preAttrs, const QString &codeAttrs)
{
    static const QRegularExpression langRe(QStringLiteral("\\blanguage-([A-Za-z0-9_.+-]+)"));
    QRegularExpressionMatch m = langRe.match(codeAttrs);
    if (!m.hasMatch())
        m = langRe.match(preAttrs);
    return m.hasMatch() ? m.captured(1) : QStringLiteral("text");
}

// QTextDocument::toHtml() already HTML-escapes code content; highlighters
// expect raw text, so undo the escaping first. There is no QString::fromHtml,
// so do a minimal entity replacement. "&amp;" must be replaced LAST: in
// "&amp;lt;" the other replacements must not see the "&lt;" produced by
// unescaping "&amp;" (that would double-unescape to "<").
QString unescapeHtml(const QString &escaped)
{
    QString s = escaped;
    s.replace(QStringLiteral("&lt;"), QLatin1String("<"));
    s.replace(QStringLiteral("&gt;"), QLatin1String(">"));
    s.replace(QStringLiteral("&quot;"), QLatin1String("\""));
    s.replace(QStringLiteral("&amp;"), QLatin1String("&"));
    return s;
}

// Wraps <pre>... blocks in a container with a header bar (language label +
// copy button). Handles both "<pre><code>..." (rich parsers that tag code
// elements with class="language-xxx") and plain "<pre>..." (QTextDocument).
// QString has no replace(QRegularExpression, lambda) overload, so we rebuild
// the string manually over successive matches.
QString wrapCodeBlocks(const QString &bodyHtml, const HtmlGenerationOptions &options)
{
    static const QRegularExpression blockRe(
        QStringLiteral("<pre([^>]*)>(?:\\s*<code([^>]*)>)?(.*?)(?:</code>\\s*)?</pre>"),
        QRegularExpression::DotMatchesEverythingOption);

    QString out;
    int last = 0;
    int codeId = 0;
    QRegularExpressionMatch m;
    int pos = bodyHtml.indexOf(blockRe, last, &m);
    while (pos != -1) {
        out += bodyHtml.mid(last, pos - last);
        const QString preAttrs = m.captured(1);
        const QString codeAttrs = m.captured(2);
        const QString content = m.captured(3); // already HTML-escaped by the parser
        const QString lang = extractLanguage(preAttrs, codeAttrs);

        // When a highlighter is configured, its output replaces the inner
        // body of the <pre> block (including any <code> wrapper); the outer
        // code-block/code-header/copy-btn structure is preserved.
        QString highlighted;
        if (options.highlighter)
            highlighted = options.highlighter->highlight(unescapeHtml(content), lang, options.darkMode);

        out += QStringLiteral("<div class=\"code-block\">"
                              "<div class=\"code-header\">"
                              "<span class=\"code-lang\">%1</span>"
                              "<button class=\"copy-btn\" data-code-id=\"%2\">%3</button>"
                              "</div>"
                              "<pre%4>")
                   .arg(lang.toHtmlEscaped(), QString::number(codeId++),
                        options.codeBlockCopyButtonText.toHtmlEscaped(), preAttrs);
        if (!highlighted.isEmpty())
            out += highlighted;
        else if (!codeAttrs.isEmpty())
            out += QStringLiteral("<code%1>%2</code>").arg(codeAttrs, content);
        else
            out += content;
        out += QStringLiteral("</pre></div>");

        last = pos + m.capturedLength();
        pos = bodyHtml.indexOf(blockRe, last, &m);
    }
    out += bodyHtml.mid(last);
    return out;
}

} // namespace

QString HtmlGenerator::loadThemeCss(bool darkMode)
{
    QFile file(darkMode ? QStringLiteral(":/markdown/theme-dark.css")
                        : QStringLiteral(":/markdown/theme-light.css"));
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text))
        return QString();
    return QString::fromUtf8(file.readAll());
}

QString HtmlGenerator::generate(const MarkdownDocument &doc, const HtmlGenerationOptions &options)
{
    QString css = loadThemeCss(options.darkMode);
    if (options.formulaRenderer)
        css += QLatin1Char('\n') + options.formulaRenderer->requiredCss();
    QString bodyHtml = extractBody(doc.toHtml(options));
    bodyHtml = wrapCodeBlocks(bodyHtml, options);
    return QStringLiteral("<!DOCTYPE html><html><head><meta charset=\"utf-8\"/><style>%1</style></head>"
                          "<body>%2</body></html>")
        .arg(css, bodyHtml);
}
