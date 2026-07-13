#include "chat_message_item.h"
#include "theme_colors.h"

#include <QHBoxLayout>
#include <QFrame>

ChatMessageItem::ChatMessageItem(QWidget *parent)
    : QWidget(parent)
{
    setupLayout();
    applyStyle();
}

void ChatMessageItem::setupLayout()
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 6, 16, 6);

    m_textLabel = new QLabel(this);
    m_textLabel->setWordWrap(true);
    m_textLabel->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_textLabel->setMaximumWidth(640);

    QFrame *bubble = new QFrame(this);
    bubble->setObjectName(QStringLiteral("bubbleFrame"));
    QHBoxLayout *bubbleLayout = new QHBoxLayout(bubble);
    bubbleLayout->setContentsMargins(12, 8, 12, 8);
    bubbleLayout->addWidget(m_textLabel);

    layout->addWidget(bubble);
}

void ChatMessageItem::setMessage(const HermindChatMessage &message)
{
    m_message = message;
    m_textLabel->setText(message.content());
    applyStyle();
}

void ChatMessageItem::setDarkMode(bool dark)
{
    m_dark = dark;
    applyStyle();
}

QString ChatMessageItem::messageText() const
{
    return m_textLabel->text();
}

bool ChatMessageItem::isUserMessage() const
{
    return m_message.role() == HermindChatMessage::User;
}

void ChatMessageItem::applyStyle()
{
    QHBoxLayout *layout = qobject_cast<QHBoxLayout *>(this->layout());
    if (!layout || layout->count() == 0)
        return;

    QFrame *bubble = qobject_cast<QFrame *>(layout->itemAt(0)->widget());
    if (!bubble)
        return;

    const bool user = isUserMessage();
    layout->setAlignment(user ? Qt::AlignRight : Qt::AlignLeft);

    const QColor bg = user ? ThemeColors::primary(m_dark)
                           : ThemeColors::cardBackground(m_dark);
    const QColor fg = user ? QColor(255, 255, 255)
                           : ThemeColors::textPrimary(m_dark);

    bubble->setStyleSheet(QStringLiteral(
        "#bubbleFrame { background-color: %1; border-radius: 12px; }"
    ).arg(bg.name()));
    m_textLabel->setStyleSheet(QStringLiteral("color: %1; font-size: 14px;").arg(fg.name()));
}
