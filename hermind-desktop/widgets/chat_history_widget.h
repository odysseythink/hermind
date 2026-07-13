#ifndef CHAT_HISTORY_WIDGET_H
#define CHAT_HISTORY_WIDGET_H

#include <QWidget>
#include <QScrollArea>
#include <QVBoxLayout>
#include <QVector>
#include "hermind_chat_message.h"

class QLabel;

class ChatHistoryWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ChatHistoryWidget(QWidget *parent = nullptr);

    void setMessages(const QVector<HermindChatMessage> &messages);
    void appendMessage(const HermindChatMessage &message);
    void updateMessage(int index, const HermindChatMessage &message);
    void clear();

    int messageCount() const;
    bool isAtBottom() const;

    void setWelcomeText(const QString &text);

signals:
    void regenerateRequested();

private:
    void rebuild();
    void appendItem(const HermindChatMessage &message);
    void scrollToBottom();
    void applyTheme();

    QScrollArea *m_scrollArea = nullptr;
    QWidget *m_container = nullptr;
    QVBoxLayout *m_layout = nullptr;
    QLabel *m_welcomeLabel = nullptr;
    QVector<HermindChatMessage> m_messages;
    QVector<QWidget *> m_items;
};

#endif // CHAT_HISTORY_WIDGET_H
