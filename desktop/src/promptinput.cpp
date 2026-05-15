#include "promptinput.h"

#include <QTextEdit>
#include <QPushButton>
#include <QHBoxLayout>
#include <QVBoxLayout>

PromptInput::PromptInput(QWidget *parent)
    : QWidget(parent),
      m_textEdit(new QTextEdit(this)),
      m_sendBtn(new QPushButton("Send", this)),
      m_attachBtn(new QPushButton("Attach", this))
{
    m_textEdit->setPlaceholderText("Type a message...");
    m_textEdit->setMaximumHeight(120);
    m_textEdit->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);

    m_sendBtn->setStyleSheet(
        "QPushButton { background: #FFB800; color: #0a0b0d; border: 1px solid #FFB800; "
        "border-radius: 4px; padding: 6px 16px; font-weight: 600; font-size: 12px; }"
        "QPushButton:hover { background: #FF8A00; border-color: #FF8A00; }"
    );

    m_attachBtn->setStyleSheet(
        "QPushButton { background: transparent; color: #8a8680; border: 1px solid #2a2e36; "
        "border-radius: 4px; padding: 6px 14px; font-size: 12px; }"
        "QPushButton:hover { border-color: #FFB800; color: #e8e6e3; }"
    );

    QHBoxLayout *buttonLayout = new QHBoxLayout;
    buttonLayout->addWidget(m_attachBtn);
    buttonLayout->addStretch(1);
    buttonLayout->addWidget(m_sendBtn);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(16, 8, 16, 16);
    layout->setSpacing(8);
    layout->addWidget(m_textEdit);
    layout->addLayout(buttonLayout);

    connect(m_sendBtn, &QPushButton::clicked,
            this, &PromptInput::onSendClicked);
    connect(m_attachBtn, &QPushButton::clicked,
            this, &PromptInput::attachClicked);
}

QString PromptInput::text() const
{
    return m_textEdit->toPlainText();
}

void PromptInput::insertText(const QString &text)
{
    m_textEdit->insertPlainText(text);
}

void PromptInput::clear()
{
    m_textEdit->clear();
}

void PromptInput::setEnabled(bool enabled)
{
    m_textEdit->setEnabled(enabled);
    m_sendBtn->setEnabled(enabled);
    m_attachBtn->setEnabled(enabled);
}

void PromptInput::onSendClicked()
{
    QString t = m_textEdit->toPlainText().trimmed();
    if (!t.isEmpty()) {
        m_textEdit->clear();
        emit sendClicked();
    }
}
