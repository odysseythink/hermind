#ifndef MARKDOWNRENDERER_H
#define MARKDOWNRENDERER_H

#include <QString>

class HermindClient;

class MarkdownRenderer
{
public:
    static QString render(const QString &markdown, HermindClient *client = nullptr);

private:
    struct Segment {
        enum Type { Markdown, Code } type;
        QString language;
        QString content;
    };

    static QList<Segment> parseSegments(const QString &markdown);
    static QString renderMarkdownSegment(const QString &markdown);
    static QString renderCodeSegment(const QString &code, const QString &language);
};

#endif
