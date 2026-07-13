#include "workspace_item_widget.h"
#include "hermind_workspace.h"
#include "thread_container_widget.h"
#include "hermind_api_client.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "icon_button.h"

#include <QLabel>
#include <QMouseEvent>
#include <QVBoxLayout>
#include <QHBoxLayout>

WorkspaceItemWidget::WorkspaceItemWidget(QWidget *parent)
    : QWidget(parent)
    , m_nameLabel(new QLabel(this))
    , m_expandLabel(new QLabel(QStringLiteral("▶"), this))
    , m_header(new QWidget(this))
    , m_content(new QWidget(this))
    , m_threadContainer(new ThreadContainerWidget(m_content))
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(0);

    // Header
    auto *headerLayout = new QHBoxLayout(m_header);
    headerLayout->setContentsMargins(4, 6, 8, 6);
    headerLayout->setSpacing(6);

    m_expandLabel->setStyleSheet(QStringLiteral("font-size: 10px;"));
    m_expandLabel->setAttribute(Qt::WA_TransparentForMouseEvents);
    headerLayout->addWidget(m_expandLabel);

    m_nameLabel->setWordWrap(false);
    m_nameLabel->setAttribute(Qt::WA_TransparentForMouseEvents);
    headerLayout->addWidget(m_nameLabel, 1);

    IconButton *uploadButton = new IconButton(m_header);
    uploadButton->setIconText(QStringLiteral("↑"));
    uploadButton->setToolTip(tr("Upload documents"));
    connect(uploadButton, &IconButton::clicked, this, &WorkspaceItemWidget::onUploadClicked);
    headerLayout->addWidget(uploadButton);

    IconButton *settingsButton = new IconButton(m_header);
    settingsButton->setIconText(QStringLiteral("⚙"));
    settingsButton->setToolTip(tr("Workspace settings"));
    connect(settingsButton, &IconButton::clicked, this, &WorkspaceItemWidget::onSettingsClicked);
    headerLayout->addWidget(settingsButton);

    m_header->installEventFilter(this);
    m_header->setCursor(Qt::PointingHandCursor);

    rootLayout->addWidget(m_header);

    // Content (thread container)
    auto *contentLayout = new QVBoxLayout(m_content);
    contentLayout->setContentsMargins(0, 0, 0, 0);
    contentLayout->setSpacing(0);
    contentLayout->addWidget(m_threadContainer);
    rootLayout->addWidget(m_content);

    m_content->setVisible(false);

    connect(m_threadContainer, &ThreadContainerWidget::threadClicked,
            this, &WorkspaceItemWidget::threadClicked);

    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &WorkspaceItemWidget::applyStyle);
}

void WorkspaceItemWidget::setWorkspace(const HermindWorkspace &workspace)
{
    m_workspace = workspace;
    m_nameLabel->setText(workspace.name());
    m_threadContainer->setWorkspaceSlug(workspace.slug());
}

QString WorkspaceItemWidget::workspaceSlug() const
{
    return m_workspace.slug();
}

void WorkspaceItemWidget::setActive(bool active)
{
    if (m_active == active)
        return;
    m_active = active;
    setExpanded(m_active);
    applyStyle();
}

bool WorkspaceItemWidget::isActive() const
{
    return m_active;
}

void WorkspaceItemWidget::setExpanded(bool expanded)
{
    if (m_expanded == expanded)
        return;
    m_expanded = expanded;
    m_content->setVisible(m_expanded);
    updateExpandArrow();
    if (m_expanded)
        m_threadContainer->refresh();
}

bool WorkspaceItemWidget::isExpanded() const
{
    return m_expanded;
}

void WorkspaceItemWidget::setApiClient(HermindApiClient *apiClient)
{
    m_threadContainer->setApiClient(apiClient);
}

void WorkspaceItemWidget::setSelectedThreadSlug(const QString &slug)
{
    m_threadContainer->setSelectedThreadSlug(slug);
}

void WorkspaceItemWidget::refreshThreads()
{
    m_threadContainer->refresh();
}

void WorkspaceItemWidget::simulateClick()
{
    emit workspaceClicked(workspaceSlug());
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

bool WorkspaceItemWidget::eventFilter(QObject *watched, QEvent *event)
{
    if (watched == m_header && event->type() == QEvent::MouseButtonPress) {
        auto *me = static_cast<QMouseEvent *>(event);
        if (me->button() == Qt::LeftButton)
            emit workspaceClicked(workspaceSlug());
        return true;
    }
    return QWidget::eventFilter(watched, event);
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
        "  background-color: transparent;"
        "  border: none;"
        "}"
        "QLabel {"
        "  color: %1;"
        "  font-size: 14px;"
        "  font-weight: %2;"
        "}"
    ).arg(text, m_active ? QStringLiteral("600") : QStringLiteral("400")));

    m_header->setStyleSheet(QStringLiteral(
        "background-color: %1; border-radius: 6px;"
    ).arg(bg));
}

void WorkspaceItemWidget::onSettingsClicked()
{
    emit workspaceSettingsRequested(workspaceSlug());
}

void WorkspaceItemWidget::onUploadClicked()
{
    emit uploadDocumentsRequested(workspaceSlug());
}

void WorkspaceItemWidget::updateExpandArrow()
{
    m_expandLabel->setText(m_expanded ? QStringLiteral("▼") : QStringLiteral("▶"));
}
