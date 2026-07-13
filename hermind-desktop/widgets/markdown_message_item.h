#ifndef MARKDOWN_MESSAGE_ITEM_H
#define MARKDOWN_MESSAGE_ITEM_H

#include <QWidget>
#include <QPointer>

#include "hermind_chat_message.h"

class MarkdownRenderer;

// Assistant-message bubble. While the message is still streaming
// (!isClosed()) it shows a plain QLabel; once closed it renders the
// content through MarkdownRenderer (QTextBrowser, with QLabel fallback).
class MarkdownMessageItem : public QWidget
{
    Q_OBJECT
public:
    explicit MarkdownMessageItem(QWidget *parent = nullptr);
    ~MarkdownMessageItem() override;

    void setMessage(const HermindChatMessage &message);
    void setDarkMode(bool dark);

private:
    void showPlainText(const QString &text);
    void showMarkdown(const QString &text, bool dark);
    void applyBubbleStyle();

    HermindChatMessage m_message;
    MarkdownRenderer *m_renderer = nullptr;
    QPointer<QWidget> m_currentContent;
    bool m_dark = false;
};

#endif // MARKDOWN_MESSAGE_ITEM_H
