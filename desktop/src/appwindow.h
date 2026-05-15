#ifndef APPWINDOW_H
#define APPWINDOW_H

#include <QWidget>
#include <QSplitter>

class TopBar;
class SessionListWidget;
class ChatWidget;
class StatusFooter;
class HermindClient;
class SettingsDialog;

class AppWindow : public QWidget
{
    Q_OBJECT
public:
    explicit AppWindow(QWidget *parent = nullptr);
    void setClient(HermindClient *client);

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
    SettingsDialog *m_settingsDialog;
};

#endif
