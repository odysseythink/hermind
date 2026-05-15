#ifndef CODEHIGHLIGHTER_H
#define CODEHIGHLIGHTER_H

#include <QSyntaxHighlighter>
#include <QString>
#include <QStringList>
#include <QTextCharFormat>
#include <QVector>

class CodeHighlighter : public QSyntaxHighlighter
{
    Q_OBJECT
public:
    explicit CodeHighlighter(QTextDocument *parent = nullptr);
    void setLanguage(const QString &language);
    static QString highlightCode(const QString &code, const QString &language);

protected:
    void highlightBlock(const QString &text) override;

private:
    struct LanguageRules {
        QStringList keywords;
        QStringList commentPatterns;
        QStringList stringPatterns;
    };

    struct Range {
        int start;
        int length;
        QTextCharFormat format;
    };

    void setupRules(const QString &language);
    static bool isOverlapping(int start, int len, const QVector<Range> &ranges);

    LanguageRules m_rules;
    QTextCharFormat m_keywordFormat;
    QTextCharFormat m_stringFormat;
    QTextCharFormat m_commentFormat;
    QTextCharFormat m_numberFormat;
    QTextCharFormat m_functionFormat;
};

#endif
