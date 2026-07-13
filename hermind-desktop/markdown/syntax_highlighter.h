#pragma once

#include "i_markdown_parser.h"

// Safe fallback highlighter: performs no real syntax highlighting, it simply
// escapes the code and wraps it in <pre><code class="language-LANG">. The
// class attribute is whitelisted by HtmlSanitizer on pre/code, so the output
// survives sanitization. Empty code yields an empty string; the language
// class is only emitted for non-empty languages other than "text".
class NullSyntaxHighlighter : public ISyntaxHighlighter {
public:
    QString highlight(const QString &code, const QString &language, bool darkMode) const override;
    QStringList supportedLanguages() const override;
};
