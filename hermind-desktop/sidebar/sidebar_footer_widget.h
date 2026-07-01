#ifndef SIDEBAR_FOOTER_WIDGET_H
#define SIDEBAR_FOOTER_WIDGET_H

#include <QWidget>

class SidebarFooterWidget : public QWidget
{
    Q_OBJECT

public:
    explicit SidebarFooterWidget(QWidget *parent = nullptr);

    // 仅用于测试触发
    void clickGitHubButton();
    void clickSettingsButton();

signals:
    void openGitHubRequested();
    void openSettingsRequested();

private:
    class IconButton *m_githubButton = nullptr;
    class IconButton *m_settingsButton = nullptr;
};

#endif // SIDEBAR_FOOTER_WIDGET_H
