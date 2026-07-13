#pragma once

#include <QString>
#include <QStringList>
#include <memory>

class ISyntaxHighlighter;
class IFormulaRenderer;

struct HtmlGenerationOptions {
    bool darkMode = true;
    bool enableRawHtml = false;
    QString codeBlockCopyButtonText = "Copy";
    QString highlightTheme = "github-dark";
    QString katexCssPath = ":/katex/katex.min.css";
    ISyntaxHighlighter *highlighter = nullptr; // non-owning; null = no highlighting
    IFormulaRenderer *formulaRenderer = nullptr; // non-owning; null = no formula CSS injection
};

// Produces highlighted HTML for a single code block. Implementations must
// escape the code text and must only emit markup that survives HtmlSanitizer
// (spans/code/pre with class attributes; never style attributes).
class ISyntaxHighlighter {
public:
    virtual ~ISyntaxHighlighter() = default;
    virtual QString highlight(const QString &code, const QString &language, bool darkMode) const = 0;
    virtual QStringList supportedLanguages() const = 0;
};

// Renders a LaTeX formula to HTML. Implementations must escape the latex
// text and must only emit markup that survives HtmlSanitizer (span/div/code
// with class attributes; never style attributes).
class IFormulaRenderer {
public:
    virtual ~IFormulaRenderer() = default;
    virtual QString render(const QString &latex, bool displayMode) const = 0;
    virtual QString requiredCss() const = 0;
};

class MarkdownDocument {
public:
    virtual ~MarkdownDocument() = default;
    virtual QString toHtml(const HtmlGenerationOptions &options) const = 0;
    virtual bool isEmpty() const = 0;
};

class IMarkdownParser {
public:
    virtual ~IMarkdownParser() = default;
    virtual std::unique_ptr<MarkdownDocument> parse(const QString &markdown, QString *error = nullptr) = 0;
    virtual bool supportsGfm() const = 0;
    virtual bool supportsMath() const = 0;
};
