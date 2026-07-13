#ifndef CHAT_MESSAGE_ITEM_H
#define CHAT_MESSAGE_ITEM_H

#include <QWidget>
#include <QLabel>
#include "hermind_chat_message.h"

class ChatMessageItem : public QWidget
{
    Q_OBJECT
public:
    explicit ChatMessageItem(QWidget *parent = nullptr);

    void setMessage(const HermindChatMessage &message);
    void setDarkMode(bool dark);

    QString messageText() const;
    bool isUserMessage() const;

private:
    void setupLayout();
    void applyStyle();

    HermindChatMessage m_message;
    QLabel *m_textLabel = nullptr;
    bool m_dark = false;
};

#endif // CHAT_MESSAGE_ITEM_H
