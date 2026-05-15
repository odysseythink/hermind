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
    explicit MessageBubble(const QString &role, QWidget *parent = nullptr);

    void appendMarkdown(const QString &text);
    QString markdownBuffer() const;
    void setHtmlContent(const QString &html);

private:
    QString m_role;
    QString m_markdownBuffer;
    QLabel *m_avatarLabel;
    QTextEdit *m_textEdit;
};

#endif
