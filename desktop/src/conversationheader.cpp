#include "conversationheader.h"

#include <QLabel>
#include <QHBoxLayout>

ConversationHeader::ConversationHeader(QWidget *parent)
    : QWidget(parent),
      m_title(new QLabel(this))
{
    setFixedHeight(44);

    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 0, 16, 0);
    layout->setSpacing(0);

    m_title->setStyleSheet(
        "font-family: monospace; font-size: 12px; text-transform: uppercase; color: #8a8680;"
    );
    m_title->setText("New Conversation");

    layout->addWidget(m_title);
    layout->addStretch(1);

    setStyleSheet("border-bottom: 1px solid #2a2e36;");
}

void ConversationHeader::setTitle(const QString &title)
{
    m_title->setText(title.toUpper());
}
