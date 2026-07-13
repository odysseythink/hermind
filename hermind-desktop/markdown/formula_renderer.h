#pragma once

#include "i_markdown_parser.h"

// Fallback formula renderer: performs no real math typesetting, it simply
// escapes the latex and wraps it in <code class="math-inline"> (inline) or
// <div class="math-block"><code>...</code></div> (display mode). The class
// attributes are whitelisted by HtmlSanitizer, so the output survives
// sanitization. Empty latex yields an empty string.
class NullFormulaRenderer : public IFormulaRenderer {
public:
    QString render(const QString &latex, bool displayMode) const override;
    QString requiredCss() const override;
};

// KaTeX-based formula renderer (offline / degraded mode): wraps the escaped
// latex in KaTeX's expected container markup (<span class="katex"> for
// inline, <div class="katex-display"> for display) and provides the KaTeX
// CSS from Qt resources. Note: without the KaTeX JS runtime the latex source
// is shown as-is, styled by the CSS — this is the documented
// fallback-quality path (QTextBrowser cannot run JS, and the CSS's web fonts
// referenced via url() are not loaded either).
class KatexFormulaRenderer : public IFormulaRenderer {
public:
    QString render(const QString &latex, bool displayMode) const override;
    QString requiredCss() const override;
};
