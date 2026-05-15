#include "messagebubble.h"
#include "toolcallwidget.h"

#include <QTextEdit>
#include <QLabel>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QPushButton>
#include <QScrollBar>
#include <QClipboard>
#include <QApplication>
#include <QDebug>

MessageBubble::MessageBubble(bool isUser, QWidget *parent)
    : QWidget(parent),
      m_isUser(isUser),
      m_editable(false),
      m_roleTag(new QLabel(this)),
      m_content(new QTextEdit(this)),
      m_operationsBar(nullptr),
      m_operationsLayout(nullptr),
      m_toolCallArea(nullptr)
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

    // Tool call area (hidden by default)
    m_toolCallArea = new QWidget(this);
    m_toolCallArea->setVisible(false);
    QVBoxLayout *toolLayout = new QVBoxLayout(m_toolCallArea);
    toolLayout->setContentsMargins(0, 0, 0, 0);
    toolLayout->setSpacing(4);

    // Operations bar (only for assistant bubbles)
    if (!m_isUser) {
        setupOperations();
    }

    QVBoxLayout *bubbleLayout = new QVBoxLayout;
    bubbleLayout->setContentsMargins(12, 10, 12, 10);
    bubbleLayout->setSpacing(4);
    bubbleLayout->addWidget(m_content);
    bubbleLayout->addWidget(m_toolCallArea);
    if (m_operationsBar) {
        bubbleLayout->addWidget(m_operationsBar);
    }

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

void MessageBubble::setupOperations()
{
    m_operationsBar = new QWidget(this);
    m_operationsLayout = new QHBoxLayout(m_operationsBar);
    m_operationsLayout->setContentsMargins(0, 4, 0, 0);
    m_operationsLayout->setSpacing(8);
    m_operationsLayout->addStretch(1);

    auto makeButton = [this](const QString &text) -> QPushButton* {
        QPushButton *btn = new QPushButton(text, m_operationsBar);
        btn->setFlat(true);
        btn->setCursor(Qt::PointingHandCursor);
        btn->setStyleSheet(
            "QPushButton {"
            "  color: #8a8680;"
            "  font-size: 11px;"
            "  padding: 2px 8px;"
            "  border: none;"
            "  background: transparent;"
            "}"
            "QPushButton:hover {"
            "  color: #e8e6e3;"
            "  background: #2a2e36;"
            "  border-radius: 3px;"
            "}"
        );
        return btn;
    };

    QPushButton *copyBtn = makeButton("Copy");
    QPushButton *regenBtn = makeButton("Regenerate");
    QPushButton *deleteBtn = makeButton("Delete");

    connect(copyBtn, &QPushButton::clicked, this, &MessageBubble::onCopyClicked);
    connect(regenBtn, &QPushButton::clicked, this, &MessageBubble::onRegenerateClicked);
    connect(deleteBtn, &QPushButton::clicked, this, &MessageBubble::onDeleteClicked);

    m_operationsLayout->addWidget(copyBtn);
    m_operationsLayout->addWidget(regenBtn);
    m_operationsLayout->addWidget(deleteBtn);
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

bool MessageBubble::isUser() const
{
    return m_isUser;
}

void MessageBubble::setMessageId(const QString &id)
{
    m_messageId = id;
}

QString MessageBubble::messageId() const
{
    return m_messageId;
}

void MessageBubble::setOperationsEnabled(bool enabled)
{
    if (m_operationsBar) {
        m_operationsBar->setVisible(enabled);
    }
}

void MessageBubble::setEditable(bool editable)
{
    m_editable = editable;
}

void MessageBubble::addToolCall(const QString &id, const QString &name, const QString &status)
{
    if (m_toolCalls.contains(id))
        return;

    ToolCallWidget *tc = new ToolCallWidget(name, status, m_toolCallArea);
    QVBoxLayout *layout = qobject_cast<QVBoxLayout*>(m_toolCallArea->layout());
    if (layout) {
        layout->addWidget(tc);
    }
    m_toolCalls.insert(id, tc);
    m_toolCallArea->setVisible(true);
}

void MessageBubble::updateToolCall(const QString &id, const QString &status)
{
    ToolCallWidget *tc = m_toolCalls.value(id);
    if (tc) {
        tc->setStatus(status);
    }
}

void MessageBubble::onCopyClicked()
{
    QClipboard *clipboard = QApplication::clipboard();
    clipboard->setText(m_markdownBuffer);
    emit copyClicked();
}

void MessageBubble::onEditClicked()
{
    emit editClicked();
}

void MessageBubble::onDeleteClicked()
{
    emit deleteClicked();
}

void MessageBubble::onRegenerateClicked()
{
    emit regenerateClicked();
}
