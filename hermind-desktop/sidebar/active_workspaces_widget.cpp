#include "active_workspaces_widget.h"
#include "workspace_item_widget.h"
#include "hermind_api_client.h"
#include "hermind_workspace.h"
#include "navigation_manager.h"
#include "navigation_route.h"
#include "api_response.h"

#include <QVBoxLayout>
#include <QLabel>
#include <QDebug>

ActiveWorkspacesWidget::ActiveWorkspacesWidget(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    m_layout = new QVBoxLayout(this);
    m_layout->setContentsMargins(0, 0, 0, 0);
    m_layout->setSpacing(4);
    m_layout->addStretch();
}

void ActiveWorkspacesWidget::refresh()
{
    if (!m_apiClient)
        return;
    m_apiClient->listWorkspaces(
        [this](const QVector<HermindWorkspace> &workspaces, const ApiError &error) {
            onWorkspacesLoaded(workspaces, error);
        });
}

void ActiveWorkspacesWidget::setSelectedSlug(const QString &slug)
{
    m_selectedSlug = slug;
    rebuildItems();
}

void ActiveWorkspacesWidget::onWorkspacesLoaded(const QVector<HermindWorkspace> &workspaces,
                                                const ApiError &error)
{
    if (!error.isEmpty()) {
        qWarning() << "Failed to load workspaces:" << error.message();
        return;
    }
    m_workspaces = workspaces;
    rebuildItems();
}

void ActiveWorkspacesWidget::onWorkspaceClicked(const QString &slug)
{
    NavigationRoute route;
    route.page = NavigationPage::WorkspaceChat;
    route.workspaceSlug = slug;
    NavigationManager::instance().navigateTo(route);
}

void ActiveWorkspacesWidget::rebuildItems()
{
    // 移除旧项（保留 stretch）
    while (m_layout->count() > 1) {
        QLayoutItem *item = m_layout->takeAt(0);
        if (item->widget())
            item->widget()->deleteLater();
        delete item;
    }

    for (const HermindWorkspace &ws : std::as_const(m_workspaces)) {
        auto *item = new WorkspaceItemWidget(this);
        item->setWorkspace(ws);
        item->setActive(ws.slug() == m_selectedSlug);
        connect(item, &WorkspaceItemWidget::workspaceClicked,
                this, &ActiveWorkspacesWidget::onWorkspaceClicked);
        m_layout->insertWidget(m_layout->count() - 1, item);
    }
}
