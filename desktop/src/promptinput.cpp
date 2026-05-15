#include "promptinput.h"

#include <QTextEdit>
#include <QPushButton>
#include <QHBoxLayout>
#include <QVBoxLayout>

PromptInput::PromptInput(QWidget *parent)
    : QWidget(parent),
      m_textEdit(new QTextEdit(this)),
      m_sendButton(new QPushButton("Send", this)),
      m_attachButton(new QPushButton("Attach", this))
{
    m_textEdit->setPlaceholderText("Type a message...");
    m_textEdit->setMaximumHeight(120);
    m_textEdit->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);

    QHBoxLayout *buttonLayout = new QHBoxLayout;
    buttonLayout->addWidget(m_attachButton);
    buttonLayout->addStretch(1);
    buttonLayout->addWidget(m_sendButton);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 4, 8, 8);
    layout->setSpacing(4);
    layout->addWidget(m_textEdit);
    layout->addLayout(buttonLayout);

    connect(m_sendButton, &QPushButton::clicked,
            this, &PromptInput::onSendClicked);
    connect(m_attachButton, &QPushButton::clicked,
            this, &PromptInput::attachClicked);
}

void PromptInput::onSendClicked()
{
    QString text = m_textEdit->toPlainText().trimmed();
    if (!text.isEmpty()) {
        emit sendClicked(text);
        m_textEdit->clear();
    }
}
