#include "markdown_message_item.h"

#include "markdown_renderer.h"
#include "theme_colors.h"

#include <QAbstractScrollArea>
#include <QFrame>
#include <QGuiApplication>
#include <QClipboard>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QScrollBar>
#include <QTimer>
#include <QVBoxLayout>

MarkdownMessageItem::MarkdownMessageItem(QWidget *parent)
    : QWidget(parent)
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 6, 16, 6);
    layout->setAlignment(Qt::AlignLeft);
}

MarkdownMessageItem::~MarkdownMessageItem()
{
    // Delete the renderer (and the widget it currently owns) before the
    // widget tree is torn down, so the renderer never deletes a widget
    // that was already destroyed via its reparented bubble.
    delete m_renderer;
    m_renderer = nullptr;
}

void MarkdownMessageItem::setMessage(const HermindChatMessage &message)
{
    m_message = message;
    if (!message.isClosed())
        showPlainText(message.content());
    else
        showMarkdown(message.content(), m_dark);
}

void MarkdownMessageItem::setDarkMode(bool dark)
{
    m_dark = dark;
    if (m_message.isClosed() && m_renderer)
        m_renderer->setDarkMode(dark);
    applyBubbleStyle();
}

void MarkdownMessageItem::showPlainText(const QString &text)
{
    if (m_currentContent) {
        if (m_renderer && m_currentContent == m_renderer->widget()) {
            // The renderer owns this widget; detach it and let the
            // renderer dispose of it on its next render / destruction.
            m_currentContent->setParent(nullptr);
            m_currentContent->hide();
        } else {
            m_currentContent->deleteLater();
        }
    }

    QLabel *label = new QLabel(text);
    label->setWordWrap(true);
    label->setTextInteractionFlags(Qt::TextSelectableByMouse);
    label->setMaximumWidth(640);
    m_currentContent = label;

    applyBubbleStyle();
}

void MarkdownMessageItem::showMarkdown(const QString &text, bool dark)
{
    // Best-effort scroll preservation: the content swap can shift the
    // document height of an enclosing scroll area.
    QPointer<QAbstractScrollArea> scrollArea;
    int scrollValue = 0;
    for (QWidget *p = parentWidget(); p; p = p->parentWidget()) {
        if (auto *area = qobject_cast<QAbstractScrollArea *>(p)) {
            scrollArea = area;
            scrollValue = area->verticalScrollBar()->value();
            break;
        }
    }

    if (m_currentContent && (!m_renderer || m_currentContent != m_renderer->widget()))
        m_currentContent->deleteLater();

    if (!m_renderer) {
        m_renderer = new MarkdownRenderer(this);
        connect(m_renderer, &MarkdownRenderer::renderFailed,
                this, [](const QString &reason) {
                    qWarning() << "MarkdownMessageItem: render failed:" << reason;
                });
    }
    m_renderer->setMarkdown(text, dark);
    // Re-fetch after every setMarkdown(): the renderer may deleteLater()
    // and recreate its internal widget, or fall back to a QLabel.
    m_currentContent = m_renderer->widget();

    applyBubbleStyle();

    if (scrollArea) {
        QTimer::singleShot(0, this, [scrollArea, scrollValue]() {
            if (!scrollArea)
                return;
            QScrollBar *bar = scrollArea->verticalScrollBar();
            bar->setValue(qMin(scrollValue, bar->maximum()));
        });
    }
}

void MarkdownMessageItem::copyToClipboard()
{
    QGuiApplication::clipboard()->setText(m_message.content());
}

void MarkdownMessageItem::applyBubbleStyle()
{
    QHBoxLayout *outer = qobject_cast<QHBoxLayout *>(layout());
    if (!outer)
        return;

    // Rebuild the bubble frame around the current content widget.
    while (outer->count() > 0) {
        QLayoutItem *item = outer->takeAt(0);
        if (QWidget *w = item->widget())
            w->deleteLater();
        delete item;
    }

    if (!m_currentContent)
        return;

    QFrame *bubble = new QFrame(this);
    bubble->setObjectName(QStringLiteral("bubbleFrame"));
    QVBoxLayout *bubbleLayout = new QVBoxLayout(bubble);
    bubbleLayout->setContentsMargins(12, 8, 12, 8);

    m_currentContent->setParent(bubble);
    bubbleLayout->addWidget(m_currentContent);
    m_currentContent->show();

    // Action row: copy + regenerate (regenerate only once the message is final)
    QHBoxLayout *actions = new QHBoxLayout();
    actions->setContentsMargins(0, 4, 0, 0);
    QPushButton *copyBtn = new QPushButton(QStringLiteral("复制"), bubble);
    copyBtn->setObjectName(QStringLiteral("copyBtn"));
    copyBtn->setFlat(true);
    copyBtn->setCursor(Qt::PointingHandCursor);
    connect(copyBtn, &QPushButton::clicked, this, &MarkdownMessageItem::copyToClipboard);
    actions->addWidget(copyBtn);

    QPushButton *regenBtn = new QPushButton(QStringLiteral("重新生成"), bubble);
    regenBtn->setObjectName(QStringLiteral("regenerateBtn"));
    regenBtn->setFlat(true);
    regenBtn->setCursor(Qt::PointingHandCursor);
    regenBtn->setVisible(m_message.isClosed());
    connect(regenBtn, &QPushButton::clicked, this, &MarkdownMessageItem::regenerateRequested);
    actions->addWidget(regenBtn);
    actions->addStretch();
    bubbleLayout->addLayout(actions);

    outer->addWidget(bubble);

    bubble->setStyleSheet(QStringLiteral(
        "#bubbleFrame { background-color: %1; border-radius: 12px; }"
    ).arg(ThemeColors::cardBackground(m_dark).name()));
    m_currentContent->setStyleSheet(QStringLiteral(
        "background: transparent; color: %1;"
    ).arg(ThemeColors::textPrimary(m_dark).name()));
}
