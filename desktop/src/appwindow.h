#ifndef APPWINDOW_H
#define APPWINDOW_H

#include <QMainWindow>
#include <QSplitter>

class SessionListWidget;
class ChatWidget;
class HermindClient;

class AppWindow : public QMainWindow
{
    Q_OBJECT
public:
    explicit AppWindow(QWidget *parent = nullptr);
    void setClient(HermindClient *client);

protected:
    void closeEvent(QCloseEvent *event) override;

private:
    QSplitter *m_splitter;
    SessionListWidget *m_sessionList;
    ChatWidget *m_chatWidget;
};

#endif
