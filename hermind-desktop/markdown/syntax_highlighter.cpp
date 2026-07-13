#include "syntax_highlighter.h"

QString NullSyntaxHighlighter::highlight(const QString &code, const QString &language, bool) const
{
    if (code.isEmpty())
        return QString();

    QString out = QStringLiteral("<pre><code");
    if (!language.isEmpty() && language != QStringLiteral("text"))
        out += QStringLiteral(" class=\"language-%1\"").arg(language.toHtmlEscaped());
    out += QLatin1Char('>');
    out += code.toHtmlEscaped();
    out += QStringLiteral("</code></pre>");
    return out;
}

QStringList NullSyntaxHighlighter::supportedLanguages() const
{
    return {};
}
