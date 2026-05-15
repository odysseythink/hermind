#include "markdownrenderer.h"
#include "codehighlighter.h"
#include <QTextDocument>
#include <QRegularExpression>

QList<MarkdownRenderer::Segment> MarkdownRenderer::parseSegments(const QString &markdown)
{
    QList<Segment> segments;
    QStringList lines = markdown.split('\n');
    bool inCodeBlock = false;
    QString currentLanguage;
    QString currentContent;
    Segment::Type currentType = Segment::Markdown;

    for (int i = 0; i < lines.size(); ++i) {
        QString line = lines[i];
        if (line.startsWith(QStringLiteral("```"))) {
            if (!inCodeBlock) {
                // Start of code block
                if (!currentContent.isEmpty()) {
                    Segment seg;
                    seg.type = currentType;
                    seg.content = currentContent;
                    segments.append(seg);
                }
                inCodeBlock = true;
                currentType = Segment::Code;
                currentLanguage = line.mid(3).trimmed();
                currentContent.clear();
            } else {
                // End of code block
                Segment seg;
                seg.type = Segment::Code;
                seg.language = currentLanguage;
                seg.content = currentContent;
                segments.append(seg);
                inCodeBlock = false;
                currentType = Segment::Markdown;
                currentContent.clear();
            }
        } else {
            if (!currentContent.isEmpty())
                currentContent.append('\n');
            currentContent.append(line);
        }
    }

    // Remaining content
    if (!currentContent.isEmpty()) {
        Segment seg;
        seg.type = currentType;
        seg.language = currentLanguage;
        seg.content = currentContent;
        segments.append(seg);
    }

    return segments;
}

QString MarkdownRenderer::renderMarkdownSegment(const QString &markdown)
{
    QString md = markdown;

    // Replace block math formulas with placeholders before markdown parsing
    QRegularExpression blockRe(QStringLiteral("\\$\\$(.*?)\\$\\$"));
    QRegularExpression inlineRe(QStringLiteral("(?<!\\$)\\$([^\\$\\n]+?)\\$(?!\\$)"));

    md.replace(blockRe, QStringLiteral("[[MATHBLOCK]]"));
    md.replace(inlineRe, QStringLiteral("[[MATHINLINE]]"));

    QTextDocument doc;
    doc.setMarkdown(md);
    QString html = doc.toHtml();

    // Extract body content
    QRegularExpression bodyRe(QStringLiteral("<body[^>]*>(.*)</body>"),
                              QRegularExpression::DotMatchesEverythingOption);
    auto match = bodyRe.match(html);
    if (match.hasMatch()) {
        html = match.captured(1).trimmed();
    }

    // Replace placeholders with styled HTML
    html.replace(QStringLiteral("[[MATHBLOCK]]"),
                 QStringLiteral("<div style=\"color:#569cd6;border:1px dashed #569cd6;padding:8px;"
                                "margin:8px 0;border-radius:4px;text-align:center;\">[[Math Block]]</div>"));
    html.replace(QStringLiteral("[[MATHINLINE]]"),
                 QStringLiteral("<span style=\"color:#569cd6;\">[[Math]]</span>"));

    return html;
}

QString MarkdownRenderer::renderCodeSegment(const QString &code, const QString &language)
{
    if (language == QStringLiteral("mermaid")) {
        QString escaped = code.toHtmlEscaped();
        return QStringLiteral(
            "<pre style=\"background:#1e1e1e;padding:12px;border-radius:4px;"
            "overflow-x:auto;font-family:monospace;font-size:13px;"
            "color:#d4d4d4;\"><code>%1</code></pre>"
            "<div style=\"margin-top:4px;\">"
            "<button style=\"background:#2a2e36;border:1px solid #3a3e46;"
            "color:#e8e6e3;padding:4px 12px;border-radius:4px;cursor:pointer;\""
            " disabled>View Diagram</button></div>"
        ).arg(escaped);
    }

    QString highlighted = CodeHighlighter::highlightCode(code, language);
    return QStringLiteral(
        "<pre style=\"background:#1e1e1e;padding:12px;border-radius:4px;"
        "overflow-x:auto;font-family:monospace;font-size:13px;"
        "color:#d4d4d4;\"><code>%1</code></pre>"
    ).arg(highlighted);
}

QString MarkdownRenderer::render(const QString &markdown, HermindClient *client)
{
    Q_UNUSED(client)

    auto segments = parseSegments(markdown);
    QStringList parts;

    for (const auto &seg : segments) {
        if (seg.type == Segment::Code) {
            parts.append(renderCodeSegment(seg.content, seg.language));
        } else {
            parts.append(renderMarkdownSegment(seg.content));
        }
    }

    return parts.join(QStringLiteral("\n"));
}
