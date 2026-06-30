#ifndef MAIN_CHAT_WIDGET_H
#define MAIN_CHAT_WIDGET_H

#include <QWidget>

namespace Ui {
class MainChatWidget;
}

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
    void on_bottomChatButton_clicked();
    void on_bottomDocsButton_clicked();
    void on_bottomGithubButton_clicked();
    void on_bottomSettingButton_clicked();
    void on_headerSettingsButton_clicked();
    void on_toolsButton_clicked();
    void on_micButton_clicked();
    void on_sendButton_clicked();
    void on_createAgentButton_clicked();
    void on_editWorkspaceButton_clicked();
    void on_uploadFileButton_clicked();

private:
    void setupStyleSheet();
    void setupLogo();
    void setupThreadList();

    Ui::MainChatWidget *ui;
};

#endif // MAIN_CHAT_WIDGET_H
