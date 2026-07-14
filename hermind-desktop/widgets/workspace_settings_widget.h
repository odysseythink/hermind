#ifndef WORKSPACE_SETTINGS_WIDGET_H
#define WORKSPACE_SETTINGS_WIDGET_H

#include <QHash>
#include <QWidget>

#include "api_response.h"
#include "hermind_workspace.h"

class HermindApiClient;
class QLabel;
class QStackedWidget;
class QButtonGroup;
class SidebarMenuButton;

class WorkspaceSettingsWidget : public QWidget
{
    Q_OBJECT

public:
    explicit WorkspaceSettingsWidget(HermindApiClient *apiClient,
                                     QWidget *parent = nullptr);

    QString workspaceSlug() const;
    QString currentTabId() const;
    void setTabWidget(const QString &tabId, QWidget *widget);

public slots:
    void setWorkspaceSlug(const QString &slug);
    void setActiveTab(const QString &tabId);
    void setUserRole(const QString &role);
    void setWorkspaceDisplayName(const QString &name);

signals:
    void returnClicked();
    void tabChanged(const QString &tabId);
    void workspaceLoaded(bool success);

private slots:
    void onTabButtonClicked();
    void onWorkspacesLoaded(const QVector<HermindWorkspace> &workspaces,
                            const ApiError &error);
    void applyStyle();

private:
    void buildUi();
    void loadWorkspace();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    HermindWorkspace m_workspace;

    QStackedWidget *m_contentStack = nullptr;
    QLabel *m_workspaceNameLabel = nullptr;
    QLabel *m_headerTitleLabel = nullptr;
    QButtonGroup *m_tabGroup = nullptr;
    QHash<QString, SidebarMenuButton *> m_tabButtons;
    QString m_currentTabId;
};

#endif // WORKSPACE_SETTINGS_WIDGET_H
