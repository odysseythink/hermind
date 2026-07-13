#pragma once

#include "i_markdown_parser.h"

// Wraps MarkdownDocument output into a complete, themed, well-formed XHTML
// page suitable for feeding through HtmlSanitizer and into a QTextBrowser.
class HtmlGenerator {
public:
    static QString generate(const MarkdownDocument &doc, const HtmlGenerationOptions &options);
    static QString loadThemeCss(bool darkMode); // loads :/markdown/theme-dark.css or theme-light.css from Qt resources
};
