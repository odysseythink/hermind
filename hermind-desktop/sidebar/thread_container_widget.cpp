#include "thread_container_widget.h"
#include "thread_item_widget.h"
#include "hermind_api_client.h"
#include "hermind_workspace_thread.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QMouseEvent>
#include <QPointer>
#include <QDebug>
#include <QInputDialog>
#include <QMessageBox>

ThreadContainerWidget::ThreadContainerWidget(QWidget *parent)
    : QWidget(parent)
{
    m_layout = new QVBoxLayout(this);
    m_layout->setContentsMargins(0, 4, 0, 4);
    m_layout->setSpacing(2);

    m_defaultItem = new ThreadItemWidget(this);
    m_defaultItem->setDefaultThread(true);
    connect(m_defaultItem, &ThreadItemWidget::threadClicked,
            this, &ThreadContainerWidget::onThreadClicked);

    m_newThreadButton = new QWidget(this);
    auto *btnLayout = new QHBoxLayout(m_newThreadButton);
    btnLayout->setContentsMargins(20, 6, 8, 6);
    btnLayout->setSpacing(8);

    QLabel *iconLabel = new QLabel(QStringLiteral("+"), m_newThreadButton);
    iconLabel->setStyleSheet(QStringLiteral("font-size: 14px; font-weight: 600;"));
    btnLayout->addWidget(iconLabel);

    QLabel *textLabel = new QLabel(tr("New Thread"), m_newThreadButton);
    btnLayout->addWidget(textLabel, 1);

    m_newThreadButton->setCursor(Qt::PointingHandCursor);
    m_newThreadButton->installEventFilter(this);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged, this, [this]() {
        const bool dark = ThemeManager::instance().isDarkMode();
        const QString text = ThemeColors::textPrimary(dark).name();
        m_newThreadButton->setStyleSheet(QStringLiteral(
            "color: %1; font-size: 13px; font-weight: 600; border-radius: 4px;"
        ).arg(text));
    });

    rebuildItems();
}

void ThreadContainerWidget::setApiClient(HermindApiClient *apiClient)
{
    m_apiClient = apiClient;
}

void ThreadContainerWidget::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;
    m_workspaceSlug = slug;
    m_threads.clear();
    m_defaultItem->setWorkspaceSlug(slug);
    rebuildItems();
    refresh();
}

void ThreadContainerWidget::setSelectedThreadSlug(const QString &slug)
{
    m_selectedThreadSlug = slug;
    rebuildItems();
}

void ThreadContainerWidget::refresh()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;

    m_loading = true;
    QPointer<ThreadContainerWidget> guard(this);
    m_apiClient->listThreads(m_workspaceSlug,
        [guard, this](const QVector<HermindWorkspaceThread> &threads, const ApiError &error) {
            if (!guard)
                return;
            onThreadsLoaded(threads, error);
        });
}

void ThreadContainerWidget::onThreadsLoaded(const QVector<HermindWorkspaceThread> &threads,
                                            const ApiError &error)
{
    m_loading = false;
    if (!error.isEmpty()) {
        qWarning() << "Failed to load threads:" << error.message();
        return;
    }
    m_threads = threads;
    rebuildItems();
}

void ThreadContainerWidget::onThreadClicked(const QString &workspaceSlug, const QString &threadSlug)
{
    emit threadClicked(workspaceSlug, threadSlug);
}

void ThreadContainerWidget::onNewThreadClicked()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty()) {
        emit newThreadRequested(m_workspaceSlug);
        return;
    }

    QPointer<ThreadContainerWidget> guard(this);
    m_apiClient->createThread(m_workspaceSlug,
        [guard, this](const HermindWorkspaceThread &thread, const QString &, const ApiError &error) {
            if (!guard)
                return;
            if (!error.isEmpty()) {
                qWarning() << "Failed to create thread:" << error.message();
                return;
            }
            m_threads.append(thread);
            rebuildItems();
            emit threadClicked(m_workspaceSlug, thread.slug());
        });
}

void ThreadContainerWidget::onRenameRequested(const QString &workspaceSlug, const QString &threadSlug)
{
    if (!m_apiClient)
        return;

    bool ok = false;
    const QString newName = QInputDialog::getText(this, tr("Rename Thread"),
                                                  tr("Thread name:"), QLineEdit::Normal,
                                                  QString(), &ok);
    if (!ok || newName.trimmed().isEmpty())
        return;

    QPointer<ThreadContainerWidget> guard(this);
    m_apiClient->updateThread(workspaceSlug, threadSlug, newName.trimmed(),
        [guard, this](const HermindWorkspaceThread &thread, const QString &message, const ApiError &error) {
            if (!guard)
                return;
            onThreadRenamed(thread, message, error);
        });
}

void ThreadContainerWidget::onDeleteRequested(const QString &workspaceSlug, const QString &threadSlug)
{
    const int ret = QMessageBox::question(this, tr("Delete Thread"),
                                          tr("Are you sure you want to delete this thread? All of its chats will be deleted. You cannot undo this."),
                                          QMessageBox::Yes | QMessageBox::No,
                                          QMessageBox::No);
    if (ret != QMessageBox::Yes)
        return;

    if (!m_apiClient)
        return;

    QPointer<ThreadContainerWidget> guard(this);
    m_apiClient->deleteThread(workspaceSlug, threadSlug,
        [guard, this, threadSlug](bool success, const ApiError &error) {
            if (!guard)
                return;
            if (!success) {
                qWarning() << "Failed to delete thread:" << error.message();
                return;
            }
            for (int i = 0; i < m_threads.size(); ++i) {
                if (m_threads.at(i).slug() == threadSlug) {
                    m_threads.removeAt(i);
                    break;
                }
            }
            rebuildItems();
        });
}

void ThreadContainerWidget::onThreadRenamed(const HermindWorkspaceThread &thread,
                                            const QString &,
                                            const ApiError &error)
{
    if (!error.isEmpty()) {
        qWarning() << "Failed to rename thread:" << error.message();
        return;
    }
    for (int i = 0; i < m_threads.size(); ++i) {
        if (m_threads.at(i).slug() == thread.slug()) {
            m_threads[i] = thread;
            break;
        }
    }
    rebuildItems();
}

void ThreadContainerWidget::rebuildItems()
{
    // 移除除默认项与新建按钮外的旧项
    while (m_layout->count() > 0) {
        QLayoutItem *item = m_layout->takeAt(0);
        if (item->widget() && item->widget() != m_defaultItem && item->widget() != m_newThreadButton)
            item->widget()->deleteLater();
        delete item;
    }

    m_layout->addWidget(m_defaultItem);
    m_defaultItem->setSelected(m_selectedThreadSlug.isEmpty());

    for (const HermindWorkspaceThread &thread : std::as_const(m_threads)) {
        auto *item = new ThreadItemWidget(this);
        item->setWorkspaceSlug(m_workspaceSlug);
        item->setThread(thread);
        item->setSelected(thread.slug() == m_selectedThreadSlug);
        connect(item, &ThreadItemWidget::threadClicked,
                this, &ThreadContainerWidget::onThreadClicked);
        connect(item, &ThreadItemWidget::renameRequested,
                this, &ThreadContainerWidget::onRenameRequested);
        connect(item, &ThreadItemWidget::deleteRequested,
                this, &ThreadContainerWidget::onDeleteRequested);
        m_layout->addWidget(item);
    }

    m_layout->addWidget(m_newThreadButton);
    m_newThreadButton->setVisible(!m_workspaceSlug.isEmpty());
}

bool ThreadContainerWidget::eventFilter(QObject *watched, QEvent *event)
{
    if (watched == m_newThreadButton && event->type() == QEvent::MouseButtonPress) {
        onNewThreadClicked();
        return true;
    }
    return QWidget::eventFilter(watched, event);
}
