#include "qt_builtin_parser.h"

#include <QRegularExpression>
#include <QTextDocument>

#include <utility>

namespace {

// QTextDocument::toHtml() drops the info string of fenced code blocks, so
// recover the fence languages from the original markdown (in document order)
// and tag the corresponding <pre> elements with class="language-xxx". This
// is best-effort: a bare "```" fence consumes a <pre> with no language, and
// fences are matched to <pre> elements strictly by order of appearance.
QString tagFenceLanguages(QString html, const QString &markdown)
{
    QStringList langs; // one entry per fenced block; empty string = no language
    bool inFence = false;
    static const QRegularExpression langRe(QStringLiteral("^([A-Za-z0-9_.+-]+)"));
    const QStringList lines = markdown.split(QLatin1Char('\n'));
    for (const QString &line : lines) {
        const QString trimmed = line.trimmed();
        if (!trimmed.startsWith(QStringLiteral("```")))
            continue;
        if (inFence) {
            inFence = false; // closing fence
        } else {
            inFence = true;
            const QString info = trimmed.mid(3).trimmed();
            const QRegularExpressionMatch m = langRe.match(info);
            langs.append(m.hasMatch() ? m.captured(1) : QString());
        }
    }
    if (langs.isEmpty())
        return html;

    int searchFrom = 0;
    for (const QString &lang : std::as_const(langs)) {
        const int preIdx = html.indexOf(QStringLiteral("<pre"), searchFrom);
        if (preIdx < 0)
            break;
        const int gtIdx = html.indexOf(QLatin1Char('>'), preIdx);
        if (gtIdx < 0)
            break;
        if (!lang.isEmpty()) {
            html.insert(gtIdx, QStringLiteral(" class=\"language-%1\"").arg(lang));
            searchFrom = gtIdx + 1 + QStringLiteral(" class=\"language-\"").size() + lang.size();
        } else {
            searchFrom = gtIdx + 1;
        }
    }
    return html;
}

} // namespace

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
    return std::make_unique<QtBuiltinDocument>(tagFenceLanguages(doc.toHtml(), markdown));
}

bool QtBuiltinParser::supportsGfm() const
{
    return true;
}

bool QtBuiltinParser::supportsMath() const
{
    return false;
}
