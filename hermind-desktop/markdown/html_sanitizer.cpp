#include "html_sanitizer.h"

#include <QXmlStreamReader>

namespace {

// Synthetic root so the (often fragmentary) HTML parses as well-formed XML.
const QString kWrapperTag = QStringLiteral("hermindsanitizerroot");

WhiteList defaultWhitelist()
{
    WhiteList w;
    w.allowedTags = {
        QStringLiteral("p"), QStringLiteral("h1"), QStringLiteral("h2"),
        QStringLiteral("h3"), QStringLiteral("h4"), QStringLiteral("h5"),
        QStringLiteral("h6"), QStringLiteral("ul"), QStringLiteral("ol"),
        QStringLiteral("li"), QStringLiteral("blockquote"), QStringLiteral("pre"),
        QStringLiteral("code"), QStringLiteral("table"), QStringLiteral("thead"),
        QStringLiteral("tbody"), QStringLiteral("tr"), QStringLiteral("th"),
        QStringLiteral("td"), QStringLiteral("img"), QStringLiteral("a"),
        QStringLiteral("strong"), QStringLiteral("em"), QStringLiteral("del"),
        QStringLiteral("hr"), QStringLiteral("br"), QStringLiteral("span"),
        QStringLiteral("div"), QStringLiteral("input"), QStringLiteral("html"),
        QStringLiteral("head"), QStringLiteral("body"), QStringLiteral("meta"),
        QStringLiteral("style"), QStringLiteral("link"), QStringLiteral("title"),
    };
    w.allowedAttrs[QStringLiteral("a")] = {QStringLiteral("href"), QStringLiteral("class")};
    w.allowedAttrs[QStringLiteral("img")] = {QStringLiteral("src"), QStringLiteral("alt"), QStringLiteral("class")};
    w.allowedAttrs[QStringLiteral("input")] = {QStringLiteral("type"), QStringLiteral("checked"), QStringLiteral("disabled")};
    w.allowedAttrs[QStringLiteral("pre")] = {QStringLiteral("class")};
    w.allowedAttrs[QStringLiteral("code")] = {QStringLiteral("class")};
    w.allowedAttrs[QStringLiteral("span")] = {QStringLiteral("class")};
    w.allowedAttrs[QStringLiteral("div")] = {QStringLiteral("class"), QStringLiteral("id")};
    w.allowedAttrs[QStringLiteral("link")] = {QStringLiteral("rel"), QStringLiteral("href")};
    w.allowedAttrs[QStringLiteral("meta")] = {QStringLiteral("charset"), QStringLiteral("content"), QStringLiteral("http-equiv")};
    w.allowedAttrs[QStringLiteral("")] = {QStringLiteral("class"), QStringLiteral("id")}; // global fallback
    w.allowedSchemes = {QStringLiteral("http"), QStringLiteral("https"), QStringLiteral("qrc")};
    return w;
}

bool isSafeUri(const QString &value, const QSet<QString> &allowedSchemes)
{
    const QString v = value.trimmed();
    if (v.isEmpty())
        return true;
    // Relative paths: absolute-path, dot-relative, or scheme-less.
    if (v.startsWith(QLatin1Char('/')) || v.startsWith(QLatin1String("./")) || v.startsWith(QLatin1String("../")))
        return true;
    const int colon = v.indexOf(QLatin1Char(':'));
    if (colon < 0)
        return true; // no scheme at all -> relative
    for (int i = 0; i < colon; ++i) {
        const QChar c = v.at(i);
        if (c == QLatin1Char('/') || c == QLatin1Char('?') || c == QLatin1Char('#'))
            return true; // ':' was part of the path/query, not a scheme separator
    }
    return allowedSchemes.contains(v.left(colon).toLower());
}

bool isUriAttribute(const QString &name)
{
    return name == QLatin1String("href") || name == QLatin1String("src");
}

} // namespace

QString HtmlSanitizer::sanitize(const QString &html)
{
    return sanitize(html, defaultWhitelist());
}

QString HtmlSanitizer::sanitize(const QString &html, const WhiteList &whitelist)
{
    QString result;
    QXmlStreamReader reader(QStringLiteral("<%1>").arg(kWrapperTag) + html + QStringLiteral("</%1>").arg(kWrapperTag));

    while (!reader.atEnd()) {
        reader.readNext();
        switch (reader.tokenType()) {
        case QXmlStreamReader::StartElement: {
            const QString tag = reader.name().toString().toLower();
            if (tag == kWrapperTag)
                break;
            if (!whitelist.allowedTags.contains(tag)) {
                reader.skipCurrentElement(); // drop disallowed element AND its content
                break;
            }
            result += QLatin1Char('<') + tag;

            // Resolve which attributes this tag may carry: per-tag set, else global fallback.
            auto it = whitelist.allowedAttrs.constFind(tag);
            const QSet<QString> &allowed = (it != whitelist.allowedAttrs.constEnd())
                ? it.value()
                : whitelist.allowedAttrs[QString()];

            const QXmlStreamAttributes attrs = reader.attributes();
            for (const QXmlStreamAttribute &attr : attrs) {
                const QString name = attr.name().toString().toLower();
                if (!allowed.contains(name))
                    continue;
                const QString value = attr.value().toString();
                if (isUriAttribute(name) && !isSafeUri(value, whitelist.allowedSchemes))
                    continue;
                result += QLatin1Char(' ') + name + QStringLiteral("=\"") + value.toHtmlEscaped() + QLatin1Char('"');
            }
            result += QLatin1Char('>');
            break;
        }
        case QXmlStreamReader::EndElement: {
            const QString tag = reader.name().toString().toLower();
            if (tag == kWrapperTag)
                break;
            // Disallowed start elements were skipped entirely, so their end
            // tags never reach us; only emit end tags for allowed elements.
            if (whitelist.allowedTags.contains(tag))
                result += QStringLiteral("</") + tag + QLatin1Char('>');
            break;
        }
        case QXmlStreamReader::Characters:
            result += reader.text().toString().toHtmlEscaped();
            break;
        default:
            break;
        }
        // On any parse error atEnd() becomes true and the loop simply stops;
        // whatever was already emitted stays.
    }
    return result;
}
