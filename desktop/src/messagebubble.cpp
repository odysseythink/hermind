#include "messagebubble.h"

#include <QTextEdit>
#include <QLabel>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QScrollBar>

MessageBubble::MessageBubble(const QString &role, QWidget *parent)
    : QWidget(parent),
      m_role(role),
      m_avatarLabel(new QLabel(this)),
      m_textEdit(new QTextEdit(this))
{
    m_avatarLabel->setFixedSize(32, 32);
    m_avatarLabel->setText(role == "user" ? "U" : "A");
    m_avatarLabel->setAlignment(Qt::AlignCenter);
    m_avatarLabel->setStyleSheet(
        "background-color: #666; color: white; border-radius: 16px;"
    );

    m_textEdit->setReadOnly(true);
    m_textEdit->setFrameStyle(QFrame::NoFrame);
    m_textEdit->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_textEdit->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_textEdit->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    m_textEdit->document()->setDocumentMargin(8);

    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(8, 4, 8, 4);
    layout->setSpacing(8);

    if (role == "user") {
        layout->addStretch(1);
        layout->addWidget(m_textEdit, 0, Qt::AlignTop);
        layout->addWidget(m_avatarLabel, 0, Qt::AlignTop);
        m_textEdit->setStyleSheet(
            "background-color: #2b5278; color: white; border-radius: 12px;"
        );
    } else {
        layout->addWidget(m_avatarLabel, 0, Qt::AlignTop);
        layout->addWidget(m_textEdit, 0, Qt::AlignTop);
        layout->addStretch(1);
        m_textEdit->setStyleSheet(
            "background-color: #3a3a3a; color: #eee; border-radius: 12px;"
        );
    }
}

void MessageBubble::appendMarkdown(const QString &text)
{
    m_markdownBuffer.append(text);
}

QString MessageBubble::markdownBuffer() const
{
    return m_markdownBuffer;
}

void MessageBubble::setHtmlContent(const QString &html)
{
    m_textEdit->setHtml(html);
    // Resize to content
    m_textEdit->document()->setTextWidth(m_textEdit->viewport()->width());
    int height = static_cast<int>(m_textEdit->document()->size().height());
    m_textEdit->setMinimumHeight(height + 8);
    m_textEdit->setMaximumHeight(height + 8);
}
