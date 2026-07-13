#pragma once

#include "i_markdown_parser.h"

// MarkdownDocument backed by QTextDocument::setMarkdown(). Stores the HTML
// produced by QTextDocument::toHtml() verbatim; toHtml() ignores the options
// because QTextDocument performs no theme-aware rendering of its own.
class QtBuiltinDocument : public MarkdownDocument {
public:
    explicit QtBuiltinDocument(QString html);

    QString toHtml(const HtmlGenerationOptions &options) const override;
    bool isEmpty() const override;

private:
    QString m_html;
};

class QtBuiltinParser : public IMarkdownParser {
public:
    std::unique_ptr<MarkdownDocument> parse(const QString &markdown, QString *error = nullptr) override;
    bool supportsGfm() const override;
    bool supportsMath() const override;
};
