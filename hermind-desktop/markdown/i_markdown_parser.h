#pragma once

#include <QString>
#include <memory>

struct HtmlGenerationOptions {
    bool darkMode = true;
    bool enableRawHtml = false;
    QString codeBlockCopyButtonText = "Copy";
    QString highlightTheme = "github-dark";
    QString katexCssPath = ":/katex/katex.min.css";
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
