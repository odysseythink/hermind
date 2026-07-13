#include "chat_history_widget.h"
#include "chat_message_item.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QScrollBar>
#include <QLabel>

ChatHistoryWidget::ChatHistoryWidget(QWidget *parent)
    : QWidget(parent)
{
    QVBoxLayout *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(0);

    m_welcomeLabel = new QLabel(tr("今天我能帮您什么？"), this);
    m_welcomeLabel->setObjectName(QStringLiteral("welcomeLabel"));
    m_welcomeLabel->setAlignment(Qt::AlignCenter);
    rootLayout->addWidget(m_welcomeLabel);

    m_scrollArea = new QScrollArea(this);
    m_scrollArea->setWidgetResizable(true);
    m_scrollArea->setFrameShape(QFrame::NoFrame);
    m_scrollArea->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_scrollArea->hide();

    m_container = new QWidget;
    m_layout = new QVBoxLayout(m_container);
    m_layout->setContentsMargins(0, 12, 0, 12);
    m_layout->setSpacing(4);
    m_layout->addStretch();

    m_scrollArea->setWidget(m_container);
    rootLayout->addWidget(m_scrollArea);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ChatHistoryWidget::applyTheme);
    applyTheme();
}

void ChatHistoryWidget::setMessages(const QVector<HermindChatMessage> &messages)
{
    m_messages = messages;
    rebuild();
}

void ChatHistoryWidget::appendMessage(const HermindChatMessage &message)
{
    m_messages.append(message);
    appendItem(message);
    scrollToBottom();
}

void ChatHistoryWidget::updateMessage(int index, const HermindChatMessage &message)
{
    if (index < 0 || index >= m_messages.size())
        return;
    m_messages[index] = message;
    if (index < m_items.size())
        m_items[index]->setMessage(message);
}

void ChatHistoryWidget::clear()
{
    m_messages.clear();
    rebuild();
}

int ChatHistoryWidget::messageCount() const
{
    return m_messages.size();
}

bool ChatHistoryWidget::isAtBottom() const
{
    QScrollBar *bar = m_scrollArea->verticalScrollBar();
    if (!bar)
        return true;
    return bar->value() >= bar->maximum() - 10;
}

void ChatHistoryWidget::setWelcomeText(const QString &text)
{
    m_welcomeLabel->setText(text);
}

void ChatHistoryWidget::rebuild()
{
    for (ChatMessageItem *item : m_items) {
        m_layout->removeWidget(item);
        item->deleteLater();
    }
    m_items.clear();

    const bool hasMessages = !m_messages.isEmpty();
    m_welcomeLabel->setVisible(!hasMessages);
    m_scrollArea->setVisible(hasMessages);

    // 保留底部的 stretch
    while (m_layout->count() > 0) {
        QLayoutItem *item = m_layout->takeAt(0);
        if (item->spacerItem())
            delete item;
    }

    for (const HermindChatMessage &msg : m_messages)
        appendItem(msg);

    m_layout->addStretch();
    scrollToBottom();
}

void ChatHistoryWidget::appendItem(const HermindChatMessage &message)
{
    ChatMessageItem *item = new ChatMessageItem(m_container);
    item->setMessage(message);
    item->setDarkMode(ThemeManager::instance().isDarkMode());
    m_items.append(item);
    m_layout->insertWidget(m_layout->count() - 1, item);
}

void ChatHistoryWidget::scrollToBottom()
{
    QScrollBar *bar = m_scrollArea->verticalScrollBar();
    if (bar)
        bar->setValue(bar->maximum());
}

void ChatHistoryWidget::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    m_welcomeLabel->setStyleSheet(QStringLiteral(
        "#welcomeLabel { color: %1; font-size: 24px; font-weight: 500; }"
    ).arg(ThemeColors::textPrimary(dark).name()));
    for (ChatMessageItem *item : m_items)
        item->setDarkMode(dark);
}
