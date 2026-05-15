#include "messagebubble.h"

#include <QTextEdit>
#include <QLabel>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QScrollBar>

MessageBubble::MessageBubble(bool isUser, QWidget *parent)
    : QWidget(parent),
      m_isUser(isUser),
      m_roleTag(new QLabel(this)),
      m_content(new QTextEdit(this))
{
    setupUI();
}

void MessageBubble::setupUI()
{
    m_roleTag->setText(m_isUser ? "YOU" : "HERMIND");
    m_roleTag->setStyleSheet(
        QString("font-family: monospace; font-size: 10px; font-weight: 600; "
                "text-transform: uppercase; color: %1;")
            .arg(m_isUser ? "#FFB800" : "#8a8680")
    );

    m_content->setReadOnly(true);
    m_content->setFrameStyle(QFrame::NoFrame);
    m_content->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_content->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_content->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    m_content->document()->setDocumentMargin(12);

    QVBoxLayout *bubbleLayout = new QVBoxLayout;
    bubbleLayout->setContentsMargins(12, 10, 12, 10);
    bubbleLayout->setSpacing(4);
    bubbleLayout->addWidget(m_content);

    QWidget *bubbleWrapper = new QWidget(this);
    bubbleWrapper->setLayout(bubbleLayout);
    bubbleWrapper->setStyleSheet(
        QString("background: %1; border: 1px solid %2; border-radius: 4px;")
            .arg(m_isUser ? "transparent" : "#14161a")
            .arg(m_isUser ? "#FFB800" : "#2a2e36")
    );
    bubbleWrapper->setMaximumWidth(700);

    QVBoxLayout *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->setSpacing(2);

    if (m_isUser) {
        m_roleTag->setAlignment(Qt::AlignRight);
        outer->addWidget(m_roleTag, 0, Qt::AlignRight);

        QHBoxLayout *row = new QHBoxLayout;
        row->addStretch(1);
        row->addWidget(bubbleWrapper, 0, Qt::AlignTop);
        outer->addLayout(row);
    } else {
        m_roleTag->setAlignment(Qt::AlignLeft);
        outer->addWidget(m_roleTag, 0, Qt::AlignLeft);

        QHBoxLayout *row = new QHBoxLayout;
        row->addWidget(bubbleWrapper, 0, Qt::AlignTop);
        row->addStretch(1);
        outer->addLayout(row);
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
    m_content->setHtml(html);
    m_content->document()->setTextWidth(m_content->viewport()->width());
    int height = static_cast<int>(m_content->document()->size().height());
    m_content->setMinimumHeight(height + 8);
    m_content->setMaximumHeight(height + 8);
}
