#ifndef ACTIVE_WORKSPACES_WIDGET_H
#define ACTIVE_WORKSPACES_WIDGET_H

#include <QWidget>
#include <QVector>

#include "api_response.h"
#include "hermind_workspace.h"

class HermindApiClient;
class QVBoxLayout;
class WorkspaceItemWidget;

class ActiveWorkspacesWidget : public QWidget
{
    Q_OBJECT

public:
    explicit ActiveWorkspacesWidget(QWidget *parent = nullptr);

    void setApiClient(HermindApiClient *apiClient);
    void refresh();
    void setSelectedSlug(const QString &slug);
    void setSelectedThreadSlug(const QString &threadSlug);

private slots:
    void onWorkspacesLoaded(const QVector<HermindWorkspace> &workspaces, const ApiError &error);
    void onWorkspaceClicked(const QString &slug);
    void onThreadClicked(const QString &workspaceSlug, const QString &threadSlug);
    void onWorkspaceSettingsRequested(const QString &slug);
    void onUploadDocumentsRequested(const QString &slug);

private:
    void rebuildItems();

    HermindApiClient *m_apiClient = nullptr;
    QVector<HermindWorkspace> m_workspaces;
    QString m_selectedSlug;
    QString m_selectedThreadSlug;
    QVBoxLayout *m_layout = nullptr;
};

#endif // ACTIVE_WORKSPACES_WIDGET_H
