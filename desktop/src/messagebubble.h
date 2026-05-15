#ifndef MESSAGEBUBBLE_H
#define MESSAGEBUBBLE_H

#include <QWidget>
#include <QString>

class QTextEdit;
class QLabel;

class MessageBubble : public QWidget
{
    Q_OBJECT
public:
    explicit MessageBubble(bool isUser, QWidget *parent = nullptr);

    void appendMarkdown(const QString &text);
    QString markdownBuffer() const;
    void setHtmlContent(const QString &html);

private:
    void setupUI();

    bool m_isUser;
    QString m_markdownBuffer;
    QLabel *m_roleTag;
    QTextEdit *m_content;
};

#endif
