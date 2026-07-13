#include "plain_message_item.h"
#include "theme_colors.h"

#include <QHBoxLayout>
#include <QFrame>

PlainMessageItem::PlainMessageItem(QWidget *parent)
    : QWidget(parent)
{
    setupLayout();
    applyStyle();
}

void PlainMessageItem::setupLayout()
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 6, 16, 6);
    layout->setAlignment(Qt::AlignRight);

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

void PlainMessageItem::setMessage(const HermindChatMessage &message)
{
    m_textLabel->setText(message.content());
    applyStyle();
}

void PlainMessageItem::setDarkMode(bool dark)
{
    m_dark = dark;
    applyStyle();
}

QString PlainMessageItem::messageText() const
{
    return m_textLabel->text();
}

void PlainMessageItem::applyStyle()
{
    QHBoxLayout *layout = qobject_cast<QHBoxLayout *>(this->layout());
    if (!layout || layout->count() == 0)
        return;

    QFrame *bubble = qobject_cast<QFrame *>(layout->itemAt(0)->widget());
    if (!bubble)
        return;

    bubble->setStyleSheet(QStringLiteral(
        "#bubbleFrame { background-color: %1; border-radius: 12px; }"
    ).arg(ThemeColors::primary(m_dark).name()));
    m_textLabel->setStyleSheet(QStringLiteral("color: #ffffff; font-size: 14px;"));
}
