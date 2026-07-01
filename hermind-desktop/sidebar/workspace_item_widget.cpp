#include "workspace_item_widget.h"
#include "hermind_workspace.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QLabel>
#include <QMouseEvent>
#include <QVBoxLayout>

WorkspaceItemWidget::WorkspaceItemWidget(QWidget *parent)
    : QWidget(parent)
    , m_workspace(new HermindWorkspace())
    , m_nameLabel(new QLabel(this))
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 6, 8, 6);
    layout->addWidget(m_nameLabel);

    m_nameLabel->setWordWrap(false);
    applyStyle();

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &WorkspaceItemWidget::applyStyle);
}

void WorkspaceItemWidget::setWorkspace(const HermindWorkspace &workspace)
{
    *m_workspace = workspace;
    m_nameLabel->setText(workspace.name());
}

QString WorkspaceItemWidget::workspaceSlug() const
{
    return m_workspace ? m_workspace->slug() : QString();
}

void WorkspaceItemWidget::setActive(bool active)
{
    if (m_active == active)
        return;
    m_active = active;
    applyStyle();
}

bool WorkspaceItemWidget::isActive() const
{
    return m_active;
}

void WorkspaceItemWidget::simulateClick()
{
    emit workspaceClicked(workspaceSlug());
}

void WorkspaceItemWidget::mousePressEvent(QMouseEvent *event)
{
    if (event->button() == Qt::LeftButton)
        emit workspaceClicked(workspaceSlug());
    QWidget::mousePressEvent(event);
}

void WorkspaceItemWidget::enterEvent(QEnterEvent *event)
{
    m_hovered = true;
    applyStyle();
    QWidget::enterEvent(event);
}

void WorkspaceItemWidget::leaveEvent(QEvent *event)
{
    m_hovered = false;
    applyStyle();
    QWidget::leaveEvent(event);
}

void WorkspaceItemWidget::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString text = ThemeColors::textPrimary(dark).name();
    const QString hoverBg = ThemeColors::hoverBackground(dark).name();
    const QString selectedBg = ThemeColors::selectedBackground(dark).name();

    QString bg;
    if (m_active)
        bg = selectedBg;
    else if (m_hovered)
        bg = hoverBg;
    else
        bg = QStringLiteral("transparent");

    setStyleSheet(QStringLiteral(
        "WorkspaceItemWidget {"
        "  background-color: %1;"
        "  border-radius: 6px;"
        "}"
        "QLabel {"
        "  color: %2;"
        "  font-size: 14px;"
        "  font-weight: %3;"
        "}"
    ).arg(bg, text, m_active ? QStringLiteral("600") : QStringLiteral("400")));
}
