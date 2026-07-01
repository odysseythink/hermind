#include "thread_item_widget.h"
#include "hermind_workspace_thread.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QLabel>
#include <QMouseEvent>
#include <QVBoxLayout>
#include <QMenu>

ThreadItemWidget::ThreadItemWidget(QWidget *parent)
    : QWidget(parent)
    , m_nameLabel(new QLabel(this))
{
    auto *layout = new QHBoxLayout(this);
    layout->setContentsMargins(20, 4, 8, 4);
    layout->setSpacing(4);
    layout->addWidget(m_nameLabel, 1);

    m_nameLabel->setWordWrap(false);
    setContextMenuPolicy(Qt::DefaultContextMenu);
    applyStyle();

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ThreadItemWidget::applyStyle);
}

void ThreadItemWidget::setThread(const HermindWorkspaceThread &thread)
{
    m_threadSlug = thread.slug();
    m_name = thread.name();
    updateNameLabel();
}

void ThreadItemWidget::setWorkspaceSlug(const QString &slug)
{
    m_workspaceSlug = slug;
}

void ThreadItemWidget::setDefaultThread(bool isDefault)
{
    if (m_isDefault == isDefault)
        return;
    m_isDefault = isDefault;
    updateNameLabel();
}

void ThreadItemWidget::setSelected(bool selected)
{
    if (m_selected == selected)
        return;
    m_selected = selected;
    applyStyle();
}

bool ThreadItemWidget::isSelected() const
{
    return m_selected;
}

QString ThreadItemWidget::workspaceSlug() const
{
    return m_workspaceSlug;
}

QString ThreadItemWidget::threadSlug() const
{
    return m_isDefault ? QString() : m_threadSlug;
}

bool ThreadItemWidget::isDefaultThread() const
{
    return m_isDefault;
}

void ThreadItemWidget::simulateClick()
{
    emit threadClicked(m_workspaceSlug, threadSlug());
}

void ThreadItemWidget::simulateRenameRequest()
{
    if (!m_isDefault)
        emit renameRequested(m_workspaceSlug, m_threadSlug);
}

void ThreadItemWidget::simulateDeleteRequest()
{
    if (!m_isDefault)
        emit deleteRequested(m_workspaceSlug, m_threadSlug);
}

void ThreadItemWidget::mousePressEvent(QMouseEvent *event)
{
    if (event->button() == Qt::LeftButton)
        emit threadClicked(m_workspaceSlug, threadSlug());
    QWidget::mousePressEvent(event);
}

void ThreadItemWidget::enterEvent(QEnterEvent *event)
{
    m_hovered = true;
    applyStyle();
    QWidget::enterEvent(event);
}

void ThreadItemWidget::leaveEvent(QEvent *event)
{
    m_hovered = false;
    applyStyle();
    QWidget::leaveEvent(event);
}

void ThreadItemWidget::contextMenuEvent(QContextMenuEvent *event)
{
    if (m_isDefault)
        return;

    QMenu menu(this);
    QAction *renameAction = menu.addAction(tr("Rename"));
    QAction *deleteAction = menu.addAction(tr("Delete Thread"));

    QAction *chosen = menu.exec(event->globalPos());
    if (chosen == renameAction)
        emit renameRequested(m_workspaceSlug, m_threadSlug);
    else if (chosen == deleteAction)
        emit deleteRequested(m_workspaceSlug, m_threadSlug);
}

void ThreadItemWidget::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString text = ThemeColors::textPrimary(dark).name();
    const QString hoverBg = ThemeColors::hoverBackground(dark).name();
    const QString selectedBg = ThemeColors::selectedBackground(dark).name();

    QString bg;
    if (m_selected)
        bg = selectedBg;
    else if (m_hovered)
        bg = hoverBg;
    else
        bg = QStringLiteral("transparent");

    setStyleSheet(QStringLiteral(
        "ThreadItemWidget {"
        "  background-color: %1;"
        "  border-radius: 4px;"
        "}"
        "QLabel {"
        "  color: %2;"
        "  font-size: 13px;"
        "  font-weight: %3;"
        "}"
    ).arg(bg, text, m_selected ? QStringLiteral("600") : QStringLiteral("400")));
}

void ThreadItemWidget::updateNameLabel()
{
    m_nameLabel->setText(m_isDefault ? tr("default") : m_name);
}
