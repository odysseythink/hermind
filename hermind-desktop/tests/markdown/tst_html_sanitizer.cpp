#include <QtTest>
#include "html_sanitizer.h"

class TestHtmlSanitizer : public QObject
{
    Q_OBJECT
private slots:
    void keepsSafeTags();
    void removesScriptTag();
    void stripsJavascriptHref();
    void stripsDataScheme();
    void stripsFileScheme();
    void keepsHttpHref();
    void keepsHttpsHref();
    void keepsRelativePath();
    void keepsQrcPath();
    void removesUnknownTags();
    void stripsDangerousAttributes();
};

void TestHtmlSanitizer::keepsSafeTags()
{
    const QString in = QStringLiteral(
        "<p>Hello</p><h1>Title</h1><ul><li>one</li></ul>"
        "<blockquote>q</blockquote><pre><code class=\"language-cpp\">int x;</code></pre>"
        "<table><thead><tr><th>h</th></tr></thead><tbody><tr><td>d</td></tr></tbody></table>"
        "<strong>b</strong><em>i</em><del>s</del><hr><br><span class=\"hl\">s</span>"
        "<div id=\"d\">d</div><input type=\"checkbox\" checked=\"checked\">");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("Hello")));
    QVERIFY(out.contains(QStringLiteral("<h1")));
    QVERIFY(out.contains(QStringLiteral("<ul")));
    QVERIFY(out.contains(QStringLiteral("<li")));
    QVERIFY(out.contains(QStringLiteral("<blockquote")));
    QVERIFY(out.contains(QStringLiteral("<pre")));
    QVERIFY(out.contains(QStringLiteral("<code")));
    QVERIFY(out.contains(QStringLiteral("<table")));
    QVERIFY(out.contains(QStringLiteral("<strong")));
    QVERIFY(out.contains(QStringLiteral("<em")));
    QVERIFY(out.contains(QStringLiteral("<del")));
    QVERIFY(out.contains(QStringLiteral("<hr")));
    QVERIFY(out.contains(QStringLiteral("<br")));
    QVERIFY(out.contains(QStringLiteral("<span")));
    QVERIFY(out.contains(QStringLiteral("<div")));
    QVERIFY(out.contains(QStringLiteral("<input")));
    QVERIFY(out.contains(QStringLiteral("language-cpp")));
    QVERIFY(out.contains(QStringLiteral("id=\"d\"")));
}

void TestHtmlSanitizer::removesScriptTag()
{
    const QString in = QStringLiteral("<p>safe</p><script>alert(1)</script><p>after</p>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(!out.contains(QStringLiteral("script"), Qt::CaseInsensitive));
    QVERIFY(!out.contains(QStringLiteral("alert")));
    QVERIFY(out.contains(QStringLiteral("safe")));
    QVERIFY(out.contains(QStringLiteral("after")));
}

void TestHtmlSanitizer::stripsJavascriptHref()
{
    const QString in = QStringLiteral("<a href=\"javascript:alert(1)\">click</a>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("<a")));
    QVERIFY(!out.contains(QStringLiteral("javascript:"), Qt::CaseInsensitive));
    QVERIFY(!out.contains(QStringLiteral("href=")));
    QVERIFY(out.contains(QStringLiteral("click")));
}

void TestHtmlSanitizer::stripsDataScheme()
{
    const QString in = QStringLiteral("<a href=\"data:text/html,evil\">x</a>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("<a")));
    QVERIFY(!out.contains(QStringLiteral("data:"), Qt::CaseInsensitive));
    QVERIFY(!out.contains(QStringLiteral("href=")));
}

void TestHtmlSanitizer::stripsFileScheme()
{
    const QString in = QStringLiteral("<a href=\"file:///etc/passwd\">x</a>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("<a")));
    QVERIFY(!out.contains(QStringLiteral("file:"), Qt::CaseInsensitive));
    QVERIFY(!out.contains(QStringLiteral("href=")));
}

void TestHtmlSanitizer::keepsHttpHref()
{
    const QString in = QStringLiteral("<a href=\"http://example.com/page\">x</a>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("href=\"http://example.com/page\"")));
}

void TestHtmlSanitizer::keepsHttpsHref()
{
    const QString in = QStringLiteral("<a href=\"https://example.com/page\">x</a>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("href=\"https://example.com/page\"")));
}

void TestHtmlSanitizer::keepsRelativePath()
{
    const QString in = QStringLiteral("<img src=\"/images/logo.png\">");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("src=\"/images/logo.png\"")));
}

void TestHtmlSanitizer::keepsQrcPath()
{
    const QString in = QStringLiteral("<link href=\"qrc:///katex/katex.min.css\" rel=\"stylesheet\">");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(out.contains(QStringLiteral("href=\"qrc:///katex/katex.min.css\"")));
    QVERIFY(out.contains(QStringLiteral("rel=\"stylesheet\"")));
}

void TestHtmlSanitizer::removesUnknownTags()
{
    const QString in = QStringLiteral("<custom-tag>content</custom-tag><p>kept</p>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(!out.contains(QStringLiteral("custom-tag")));
    QVERIFY(!out.contains(QStringLiteral("content")));
    QVERIFY(out.contains(QStringLiteral("kept")));
}

void TestHtmlSanitizer::stripsDangerousAttributes()
{
    const QString in = QStringLiteral("<p onclick=\"alert(1)\" class=\"c\">text</p>");
    const QString out = HtmlSanitizer::sanitize(in);
    QVERIFY(!out.contains(QStringLiteral("onclick")));
    QVERIFY(!out.contains(QStringLiteral("alert")));
    QVERIFY(out.contains(QStringLiteral("class=\"c\"")));
    QVERIFY(out.contains(QStringLiteral("text")));
}

QTEST_APPLESS_MAIN(TestHtmlSanitizer)
#include "tst_html_sanitizer.moc"
