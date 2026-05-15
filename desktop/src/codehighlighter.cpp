#include "codehighlighter.h"
#include <QTextDocument>
#include <QRegularExpression>

CodeHighlighter::CodeHighlighter(QTextDocument *parent)
    : QSyntaxHighlighter(parent)
{
    m_keywordFormat.setForeground(QColor("#569cd6"));
    m_stringFormat.setForeground(QColor("#ce9178"));
    m_commentFormat.setForeground(QColor("#6a9955"));
    m_numberFormat.setForeground(QColor("#b5cea8"));
    m_functionFormat.setForeground(QColor("#dcdcaa"));
}

void CodeHighlighter::setLanguage(const QString &language)
{
    setupRules(language.toLower());
}

QString CodeHighlighter::highlightCode(const QString &code, const QString &language)
{
    QTextDocument doc;
    doc.setPlainText(code);

    CodeHighlighter highlighter(&doc);
    highlighter.setLanguage(language);
    highlighter.rehighlight();

    QString html = doc.toHtml();

    // Extract body content
    QRegularExpression bodyRe(QStringLiteral("<body[^>]*>(.*)</body>"),
                              QRegularExpression::DotMatchesEverythingOption);
    auto match = bodyRe.match(html);
    if (match.hasMatch()) {
        html = match.captured(1).trimmed();
    }

    // Convert <p> tags to plain divs with <br> separators
    QRegularExpression pRe(QStringLiteral("<p[^>]*>(.*?)</p>"),
                           QRegularExpression::DotMatchesEverythingOption);
    QStringList lines;
    auto it = pRe.globalMatch(html);
    while (it.hasNext()) {
        auto m = it.next();
        lines.append(m.captured(1));
    }

    if (!lines.isEmpty()) {
        return lines.join(QStringLiteral("<br>\n"));
    }
    return html;
}

void CodeHighlighter::setupRules(const QString &language)
{
    m_rules = LanguageRules();

    // Common C-family keywords
    static const QStringList cKeywords = {
        QStringLiteral("int"), QStringLiteral("char"), QStringLiteral("bool"), QStringLiteral("void"),
        QStringLiteral("return"), QStringLiteral("if"), QStringLiteral("else"), QStringLiteral("for"),
        QStringLiteral("while"), QStringLiteral("do"), QStringLiteral("switch"), QStringLiteral("case"),
        QStringLiteral("break"), QStringLiteral("continue"), QStringLiteral("default"), QStringLiteral("class"),
        QStringLiteral("struct"), QStringLiteral("enum"), QStringLiteral("typedef"), QStringLiteral("namespace"),
        QStringLiteral("template"), QStringLiteral("const"), QStringLiteral("static"), QStringLiteral("inline"),
        QStringLiteral("virtual"), QStringLiteral("override"), QStringLiteral("public"), QStringLiteral("private"),
        QStringLiteral("protected"), QStringLiteral("new"), QStringLiteral("delete"), QStringLiteral("true"),
        QStringLiteral("false"), QStringLiteral("nullptr"), QStringLiteral("auto"), QStringLiteral("using"),
        QStringLiteral("extern"), QStringLiteral("sizeof"), QStringLiteral("operator"), QStringLiteral("typename"),
        QStringLiteral("explicit"), QStringLiteral("mutable"), QStringLiteral("volatile"), QStringLiteral("register"),
        QStringLiteral("signed"), QStringLiteral("unsigned"), QStringLiteral("short"), QStringLiteral("long"),
        QStringLiteral("float"), QStringLiteral("double")
    };

    // Python keywords
    static const QStringList pyKeywords = {
        QStringLiteral("def"), QStringLiteral("class"), QStringLiteral("if"), QStringLiteral("else"),
        QStringLiteral("elif"), QStringLiteral("for"), QStringLiteral("while"), QStringLiteral("return"),
        QStringLiteral("import"), QStringLiteral("from"), QStringLiteral("as"), QStringLiteral("try"),
        QStringLiteral("except"), QStringLiteral("finally"), QStringLiteral("raise"), QStringLiteral("with"),
        QStringLiteral("pass"), QStringLiteral("break"), QStringLiteral("continue"), QStringLiteral("lambda"),
        QStringLiteral("yield"), QStringLiteral("True"), QStringLiteral("False"), QStringLiteral("None"),
        QStringLiteral("and"), QStringLiteral("or"), QStringLiteral("not"), QStringLiteral("in"),
        QStringLiteral("is"), QStringLiteral("global"), QStringLiteral("nonlocal"), QStringLiteral("assert")
    };

    // JavaScript keywords
    static const QStringList jsKeywords = {
        QStringLiteral("function"), QStringLiteral("var"), QStringLiteral("let"), QStringLiteral("const"),
        QStringLiteral("if"), QStringLiteral("else"), QStringLiteral("for"), QStringLiteral("while"),
        QStringLiteral("do"), QStringLiteral("return"), QStringLiteral("switch"), QStringLiteral("case"),
        QStringLiteral("break"), QStringLiteral("continue"), QStringLiteral("default"), QStringLiteral("class"),
        QStringLiteral("extends"), QStringLiteral("new"), QStringLiteral("this"), QStringLiteral("super"),
        QStringLiteral("import"), QStringLiteral("export"), QStringLiteral("from"), QStringLiteral("try"),
        QStringLiteral("catch"), QStringLiteral("finally"), QStringLiteral("throw"), QStringLiteral("typeof"),
        QStringLiteral("instanceof"), QStringLiteral("true"), QStringLiteral("false"), QStringLiteral("null"),
        QStringLiteral("undefined"), QStringLiteral("async"), QStringLiteral("await"), QStringLiteral("yield")
    };

    // Go keywords
    static const QStringList goKeywords = {
        QStringLiteral("package"), QStringLiteral("import"), QStringLiteral("func"), QStringLiteral("var"),
        QStringLiteral("const"), QStringLiteral("type"), QStringLiteral("struct"), QStringLiteral("interface"),
        QStringLiteral("map"), QStringLiteral("chan"), QStringLiteral("go"), QStringLiteral("defer"),
        QStringLiteral("return"), QStringLiteral("if"), QStringLiteral("else"), QStringLiteral("for"),
        QStringLiteral("range"), QStringLiteral("switch"), QStringLiteral("case"), QStringLiteral("default"),
        QStringLiteral("break"), QStringLiteral("continue"), QStringLiteral("goto"), QStringLiteral("fallthrough"),
        QStringLiteral("select"), QStringLiteral("true"), QStringLiteral("false"), QStringLiteral("nil")
    };

    // Java keywords
    static const QStringList javaKeywords = {
        QStringLiteral("public"), QStringLiteral("private"), QStringLiteral("protected"), QStringLiteral("static"),
        QStringLiteral("final"), QStringLiteral("abstract"), QStringLiteral("class"), QStringLiteral("interface"),
        QStringLiteral("extends"), QStringLiteral("implements"), QStringLiteral("import"), QStringLiteral("package"),
        QStringLiteral("void"), QStringLiteral("int"), QStringLiteral("char"), QStringLiteral("boolean"),
        QStringLiteral("byte"), QStringLiteral("short"), QStringLiteral("long"), QStringLiteral("float"),
        QStringLiteral("double"), QStringLiteral("if"), QStringLiteral("else"), QStringLiteral("for"),
        QStringLiteral("while"), QStringLiteral("do"), QStringLiteral("switch"), QStringLiteral("case"),
        QStringLiteral("break"), QStringLiteral("continue"), QStringLiteral("default"), QStringLiteral("return"),
        QStringLiteral("new"), QStringLiteral("this"), QStringLiteral("super"), QStringLiteral("try"),
        QStringLiteral("catch"), QStringLiteral("finally"), QStringLiteral("throw"), QStringLiteral("throws"),
        QStringLiteral("true"), QStringLiteral("false"), QStringLiteral("null"), QStringLiteral("instanceof"),
        QStringLiteral("enum"), QStringLiteral("assert"), QStringLiteral("synchronized"), QStringLiteral("volatile")
    };

    // Rust keywords
    static const QStringList rustKeywords = {
        QStringLiteral("fn"), QStringLiteral("let"), QStringLiteral("mut"), QStringLiteral("const"),
        QStringLiteral("static"), QStringLiteral("type"), QStringLiteral("struct"), QStringLiteral("enum"),
        QStringLiteral("trait"), QStringLiteral("impl"), QStringLiteral("pub"), QStringLiteral("use"),
        QStringLiteral("mod"), QStringLiteral("crate"), QStringLiteral("self"), QStringLiteral("super"),
        QStringLiteral("where"), QStringLiteral("unsafe"), QStringLiteral("async"), QStringLiteral("await"),
        QStringLiteral("move"), QStringLiteral("if"), QStringLiteral("else"), QStringLiteral("match"),
        QStringLiteral("for"), QStringLiteral("while"), QStringLiteral("loop"), QStringLiteral("break"),
        QStringLiteral("continue"), QStringLiteral("return"), QStringLiteral("true"), QStringLiteral("false"),
        QStringLiteral("None"), QStringLiteral("Some"), QStringLiteral("Ok"), QStringLiteral("Err")
    };

    // Shell keywords
    static const QStringList shKeywords = {
        QStringLiteral("if"), QStringLiteral("then"), QStringLiteral("else"), QStringLiteral("elif"),
        QStringLiteral("fi"), QStringLiteral("for"), QStringLiteral("while"), QStringLiteral("do"),
        QStringLiteral("done"), QStringLiteral("case"), QStringLiteral("esac"), QStringLiteral("in"),
        QStringLiteral("function"), QStringLiteral("return"), QStringLiteral("exit"), QStringLiteral("export"),
        QStringLiteral("local"), QStringLiteral("readonly"), QStringLiteral("unset"), QStringLiteral("shift"),
        QStringLiteral("echo"), QStringLiteral("printf"), QStringLiteral("test"), QStringLiteral("source"),
        QStringLiteral("alias")
    };

    // SQL keywords
    static const QStringList sqlKeywords = {
        QStringLiteral("SELECT"), QStringLiteral("FROM"), QStringLiteral("WHERE"), QStringLiteral("AND"),
        QStringLiteral("OR"), QStringLiteral("NOT"), QStringLiteral("INSERT"), QStringLiteral("UPDATE"),
        QStringLiteral("DELETE"), QStringLiteral("CREATE"), QStringLiteral("DROP"), QStringLiteral("ALTER"),
        QStringLiteral("TABLE"), QStringLiteral("INDEX"), QStringLiteral("VIEW"), QStringLiteral("JOIN"),
        QStringLiteral("INNER"), QStringLiteral("LEFT"), QStringLiteral("RIGHT"), QStringLiteral("FULL"),
        QStringLiteral("OUTER"), QStringLiteral("ON"), QStringLiteral("GROUP"), QStringLiteral("BY"),
        QStringLiteral("ORDER"), QStringLiteral("HAVING"), QStringLiteral("LIMIT"), QStringLiteral("OFFSET"),
        QStringLiteral("UNION"), QStringLiteral("ALL"), QStringLiteral("DISTINCT"), QStringLiteral("AS"),
        QStringLiteral("INTO"), QStringLiteral("VALUES"), QStringLiteral("SET"), QStringLiteral("BEGIN"),
        QStringLiteral("COMMIT"), QStringLiteral("ROLLBACK"), QStringLiteral("TRANSACTION"), QStringLiteral("PRIMARY"),
        QStringLiteral("KEY"), QStringLiteral("FOREIGN"), QStringLiteral("REFERENCES"), QStringLiteral("DEFAULT"),
        QStringLiteral("NULL"), QStringLiteral("CHECK"), QStringLiteral("UNIQUE"), QStringLiteral("CONSTRAINT")
    };

    if (language == QStringLiteral("python")) {
        m_rules.keywords = pyKeywords;
        m_rules.commentPatterns = { QStringLiteral("#.*$") };
    } else if (language == QStringLiteral("javascript") || language == QStringLiteral("js") || language == QStringLiteral("typescript") || language == QStringLiteral("ts")) {
        m_rules.keywords = jsKeywords;
        m_rules.commentPatterns = { QStringLiteral("//.*$"), QStringLiteral("/\\*.*?\\*/") };
    } else if (language == QStringLiteral("go") || language == QStringLiteral("golang")) {
        m_rules.keywords = goKeywords;
        m_rules.commentPatterns = { QStringLiteral("//.*$"), QStringLiteral("/\\*.*?\\*/") };
    } else if (language == QStringLiteral("java")) {
        m_rules.keywords = javaKeywords;
        m_rules.commentPatterns = { QStringLiteral("//.*$"), QStringLiteral("/\\*.*?\\*/") };
    } else if (language == QStringLiteral("rust")) {
        m_rules.keywords = rustKeywords;
        m_rules.commentPatterns = { QStringLiteral("//.*$"), QStringLiteral("/\\*.*?\\*/") };
    } else if (language == QStringLiteral("shell") || language == QStringLiteral("bash") || language == QStringLiteral("sh") || language == QStringLiteral("zsh")) {
        m_rules.keywords = shKeywords;
        m_rules.commentPatterns = { QStringLiteral("#.*$") };
    } else if (language == QStringLiteral("sql")) {
        m_rules.keywords = sqlKeywords;
        m_rules.commentPatterns = { QStringLiteral("--.*$"), QStringLiteral("/\\*.*?\\*/") };
    } else if (language == QStringLiteral("yaml") || language == QStringLiteral("yml")) {
        m_rules.keywords = { QStringLiteral("true"), QStringLiteral("false"), QStringLiteral("yes"), QStringLiteral("no"), QStringLiteral("null") };
        m_rules.commentPatterns = { QStringLiteral("#.*$") };
    } else if (language == QStringLiteral("json")) {
        m_rules.keywords = { QStringLiteral("true"), QStringLiteral("false"), QStringLiteral("null") };
        m_rules.commentPatterns = {};
    } else {
        // Default: C/C++ family
        m_rules.keywords = cKeywords;
        m_rules.commentPatterns = { QStringLiteral("//.*$"), QStringLiteral("/\\*.*?\\*/") };
    }

    // String patterns for all languages except SQL (which doesn't have standard string syntax in our highlighter)
    if (language != QStringLiteral("sql")) {
        m_rules.stringPatterns = {
            QStringLiteral(R"("(?:[^"\]|\.)*")"),
            QStringLiteral(R"('(?:[^'\]|\.)*')")
        };
    }
}

bool CodeHighlighter::isOverlapping(int start, int len, const QVector<Range> &ranges)
{
    int end = start + len;
    for (const auto &r : ranges) {
        int rEnd = r.start + r.length;
        if (start < rEnd && end > r.start)
            return true;
    }
    return false;
}

void CodeHighlighter::highlightBlock(const QString &text)
{
    if (m_rules.keywords.isEmpty() && m_rules.commentPatterns.isEmpty())
        return;

    QVector<Range> ranges;

    // 1. Comments (highest priority)
    for (const auto &pattern : m_rules.commentPatterns) {
        QRegularExpression re(pattern);
        auto it = re.globalMatch(text);
        while (it.hasNext()) {
            auto m = it.next();
            ranges.append({ m.capturedStart(), m.capturedLength(), m_commentFormat });
        }
    }

    // 2. Strings
    for (const auto &pattern : m_rules.stringPatterns) {
        QRegularExpression re(pattern);
        auto it = re.globalMatch(text);
        while (it.hasNext()) {
            auto m = it.next();
            ranges.append({ m.capturedStart(), m.capturedLength(), m_stringFormat });
        }
    }

    // 3. Keywords
    for (const auto &kw : m_rules.keywords) {
        QRegularExpression re(QStringLiteral("\\b%1\\b").arg(kw));
        auto it = re.globalMatch(text);
        while (it.hasNext()) {
            auto m = it.next();
            if (!isOverlapping(m.capturedStart(), m.capturedLength(), ranges)) {
                ranges.append({ m.capturedStart(), m.capturedLength(), m_keywordFormat });
            }
        }
    }

    // 4. Numbers
    {
        QRegularExpression re(QStringLiteral("\\b\\d+(?:\\.\\d+)?\\b"));
        auto it = re.globalMatch(text);
        while (it.hasNext()) {
            auto m = it.next();
            if (!isOverlapping(m.capturedStart(), m.capturedLength(), ranges)) {
                ranges.append({ m.capturedStart(), m.capturedLength(), m_numberFormat });
            }
        }
    }

    // 5. Function calls
    {
        QRegularExpression re(QStringLiteral("\\b([a-zA-Z_]\\w*)\\s*\\("));
        auto it = re.globalMatch(text);
        while (it.hasNext()) {
            auto m = it.next();
            int start = m.capturedStart(1);
            int len = m.capturedLength(1);
            if (!isOverlapping(start, len, ranges)) {
                ranges.append({ start, len, m_functionFormat });
            }
        }
    }

    // Apply all formats
    for (const auto &r : ranges) {
        setFormat(r.start, r.length, r.format);
    }
}
