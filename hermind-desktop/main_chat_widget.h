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
class ChatContainerWidget;

class MainChatWidget : public QWidget
{
    Q_OBJECT

public:
    explicit MainChatWidget(QWidget *parent = nullptr);
    ~MainChatWidget();

signals:
    void bottomSettingClicked();

private slots:
    void onNewWorkspaceRequested();
    void onRouteChanged(const NavigationRoute &route);

private:
    void setupStyleSheet();
    void replaceToolButtons();
    void replaceSidebar();
    void setupChatContainer();
    void updateSidebarSelection(const NavigationRoute &route);

    Ui::MainChatWidget *ui;
    SidebarWidget *m_sidebar = nullptr;
    ChatContainerWidget *m_chatContainer = nullptr;
};

#endif // MAIN_CHAT_WIDGET_H
