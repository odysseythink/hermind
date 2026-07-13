#include "formula_renderer.h"

#include <QFile>

QString NullFormulaRenderer::render(const QString &latex, bool displayMode) const
{
    if (latex.isEmpty())
        return QString();

    const QString escaped = latex.toHtmlEscaped();
    if (displayMode)
        return QStringLiteral("<div class=\"math-block\"><code>%1</code></div>").arg(escaped);
    return QStringLiteral("<code class=\"math-inline\">%1</code>").arg(escaped);
}

QString NullFormulaRenderer::requiredCss() const
{
    return QStringLiteral(".math-inline { font-style: italic; } "
                          ".math-block { display: block; text-align: center; margin: 12px 0; }");
}

QString KatexFormulaRenderer::render(const QString &latex, bool displayMode) const
{
    if (latex.isEmpty())
        return QString();

    const QString escaped = latex.toHtmlEscaped();
    if (displayMode)
        return QStringLiteral("<div class=\"katex-display\"><span class=\"katex\">%1</span></div>")
            .arg(escaped);
    return QStringLiteral("<span class=\"katex\">%1</span>").arg(escaped);
}

QString KatexFormulaRenderer::requiredCss() const
{
    QFile file(QStringLiteral(":/katex/katex.min.css"));
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text))
        return QString();
    return QString::fromUtf8(file.readAll());
}
