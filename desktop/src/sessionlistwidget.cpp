#include "sessionlistwidget.h"

#include <QListWidget>
#include <QPushButton>
#include <QVBoxLayout>

SessionListWidget::SessionListWidget(QWidget *parent)
    : QWidget(parent),
      m_listWidget(new QListWidget(this)),
      m_newChatButton(new QPushButton("New Chat", this))
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(8);

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
