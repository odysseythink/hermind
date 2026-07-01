#ifndef SIDEBAR_WIDGET_H
#define SIDEBAR_WIDGET_H

#include <QWidget>

namespace Ui {
class SidebarWidget;
}

class HermindApiClient;
class ActiveWorkspacesWidget;

class SidebarWidget : public QWidget
{
    Q_OBJECT

public:
    explicit SidebarWidget(QWidget *parent = nullptr);
    ~SidebarWidget();

    void setApiClient(HermindApiClient *apiClient);
    void refreshWorkspaces();
    void setSelectedWorkspace(const QString &slug);
    void setSelectedThread(const QString &threadSlug);

    // 仅用于测试触发
    void clickSettingsButton();

signals:
    void openSettingsRequested();
    void newWorkspaceRequested();

private:
    void setupLogo();
    void setupStyleSheet();

    Ui::SidebarWidget *ui;
    HermindApiClient *m_apiClient = nullptr;
};

#endif // SIDEBAR_WIDGET_H
