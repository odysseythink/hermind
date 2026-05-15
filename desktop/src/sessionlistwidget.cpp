#include "sessionlistwidget.h"

#include <QListWidget>
#include <QPushButton>
#include <QVBoxLayout>

SessionListWidget::SessionListWidget(QWidget *parent)
    : QWidget(parent),
      m_listWidget(new QListWidget(this)),
      m_newChatButton(new QPushButton("+ New Chat", this))
{
    setFixedWidth(240);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(8);

    m_newChatButton->setStyleSheet(
        "QPushButton { background: transparent; color: #8a8680; "
        "border: 1px solid #2a2e36; border-radius: 4px; padding: 6px 12px; "
        "font-family: monospace; font-size: 11px; font-weight: 600; text-transform: uppercase; }"
        "QPushButton:hover { border-color: #FFB800; color: #e8e6e3; }"
    );

    m_listWidget->setStyleSheet(
        "QListWidget { background: transparent; border: none; outline: none; }"
        "QListWidget::item { padding: 8px 16px; border-left: 2px solid transparent; color: #8a8680; }"
        "QListWidget::item:hover { background: rgba(255,255,255,0.04); }"
        "QListWidget::item:selected { background: rgba(255,184,0,0.08); color: #e8e6e3; border-left: 2px solid #FFB800; }"
    );
    m_listWidget->setFrameStyle(QFrame::NoFrame);
    m_listWidget->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);

    layout->addWidget(m_newChatButton);
    layout->addWidget(m_listWidget, 1);

    connect(m_newChatButton, &QPushButton::clicked,
            this, &SessionListWidget::onNewChatClicked);
    connect(m_listWidget, &QListWidget::itemClicked,
            this, &SessionListWidget::onItemClicked);
}

void SessionListWidget::addSession(const QString &id, const QString &title)
{
    QListWidgetItem *item = new QListWidgetItem(title, m_listWidget);
    item->setData(Qt::UserRole, id);
    m_listWidget->addItem(item);
}

void SessionListWidget::clearSessions()
{
    m_listWidget->clear();
}

void SessionListWidget::onItemClicked()
{
    QListWidgetItem *item = m_listWidget->currentItem();
    if (item) {
        emit sessionSelected(item->data(Qt::UserRole).toString());
    }
}

void SessionListWidget::onNewChatClicked()
{
    emit newSessionRequested();
}
