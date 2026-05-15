#ifndef CONVERSATIONHEADER_H
#define CONVERSATIONHEADER_H

#include <QWidget>

class QLabel;

class ConversationHeader : public QWidget
{
    Q_OBJECT
public:
    explicit ConversationHeader(QWidget *parent = nullptr);
    void setTitle(const QString &title);

private:
    QLabel *m_title;
};

#endif
