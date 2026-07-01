#ifndef MAIN_CHAT_WIDGET_H
#define MAIN_CHAT_WIDGET_H

#include <QWidget>
#include <QToolButton>

#include "navigation/navigation_route.h"

namespace Ui {
class MainChatWidget;
}

class IconButton;
class SearchInput;
class SidebarWidget;

class MainChatWidget : public QWidget
{
    Q_OBJECT

public:
    explicit MainChatWidget(QWidget *parent = nullptr);
    ~MainChatWidget();

signals:
    void bottomSettingClicked();

private slots:
    void on_popoutButton_clicked();
    void on_newSearchButton_clicked();
    void on_uploadButton_clicked();
    void on_workspaceSettingsButton_clicked();
    void on_newThreadButton_clicked();
    void on_assistantChatsButton_clicked();
    void on_bottomSettingButton_clicked();
    void on_headerSettingsButton_clicked();
    void on_toolsButton_clicked();
    void on_micButton_clicked();
    void on_sendButton_clicked();
    void on_createAgentButton_clicked();
    void on_editWorkspaceButton_clicked();
    void on_uploadFileButton_clicked();
    void onRouteChanged(const NavigationRoute &route);

private:
    void setupStyleSheet();
    void replaceToolButtons();
    void replaceSidebar();
    void updateSidebarSelection(const NavigationRoute &route);

    Ui::MainChatWidget *ui;
    SidebarWidget *m_sidebar = nullptr;
};

#endif // MAIN_CHAT_WIDGET_H
