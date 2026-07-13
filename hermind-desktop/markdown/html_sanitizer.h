#pragma once

#include <QHash>
#include <QSet>
#include <QString>

struct WhiteList {
    QSet<QString> allowedTags;
    QHash<QString, QSet<QString>> allowedAttrs; // key "" = global fallback attrs
    QSet<QString> allowedSchemes;               // for href/src URI schemes
};

class HtmlSanitizer {
public:
    static QString sanitize(const QString &html);
    static QString sanitize(const QString &html, const WhiteList &whitelist);
};
