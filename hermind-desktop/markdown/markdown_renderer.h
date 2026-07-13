#pragma once

#include <QObject>
#include <QString>
#include <QUrl>

#include <memory>

class IMarkdownParser;
class QWidget;

// Facade: markdown -> parse -> HTML generate -> sanitize -> QTextBrowser,
// with a plain-text QLabel fallback on failure. Owns the current widget;
// callers reparent the widget returned by widget() as needed.
class MarkdownRenderer : public QObject {
    Q_OBJECT
public:
    explicit MarkdownRenderer(QObject *parent = nullptr);
    ~MarkdownRenderer() override;

    void setMarkdown(const QString &markdown, bool darkMode);
    QWidget *widget() const;
    void setDarkMode(bool darkMode);

signals:
    void linkActivated(const QUrl &url);
    void copyCodeRequested(const QString &code);
    void renderFailed(const QString &reason);

private:
    void renderInternal();
    void fallbackToPlainText(const QString &text);

    std::unique_ptr<IMarkdownParser> m_parser;
    QWidget *m_currentWidget = nullptr;
    QString m_markdown;
    bool m_darkMode = true;
};
