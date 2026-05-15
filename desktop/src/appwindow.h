#ifndef APPWINDOW_H
#define APPWINDOW_H

#include <QWidget>
#include <QSplitter>

class TopBar;
class SessionListWidget;
class ChatWidget;
class StatusFooter;
class HermindClient;
class SettingsEditor;
class ThemeManager;
class I18nManager;

class AppWindow : public QWidget
{
    Q_OBJECT
public:
    explicit AppWindow(QWidget *parent = nullptr);
    void setClient(HermindClient *client);
    void setThemeManager(ThemeManager *manager);
    void setI18nManager(I18nManager *manager);

protected:
    void closeEvent(QCloseEvent *event) override;

private:
    void setupUI();
    void setupTopBar();
    void setupSidebar();
    void setupChatArea();
    void setupFooter();

    TopBar *m_topBar;
    QSplitter *m_splitter;
    SessionListWidget *m_sessionList;
    ChatWidget *m_chatWidget;
    StatusFooter *m_footer;
    SettingsEditor *m_settingsEditor;
    ThemeManager *m_themeManager;
    I18nManager *m_i18nManager;
};

#endif
