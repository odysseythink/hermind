#ifndef PLAIN_MESSAGE_ITEM_H
#define PLAIN_MESSAGE_ITEM_H

#include <QWidget>
#include <QLabel>
#include "hermind_chat_message.h"

// User-message bubble: always right-aligned with the primary accent
// background. Extracted from ChatMessageItem's user-message path.
class PlainMessageItem : public QWidget
{
    Q_OBJECT
public:
    explicit PlainMessageItem(QWidget *parent = nullptr);

    void setMessage(const HermindChatMessage &message);
    void setDarkMode(bool dark);

    QString messageText() const;

private:
    void setupLayout();
    void applyStyle();

    QLabel *m_textLabel = nullptr;
    bool m_dark = false;
};

#endif // PLAIN_MESSAGE_ITEM_H
