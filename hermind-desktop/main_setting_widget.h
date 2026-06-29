#ifndef MAIN_SETTING_WIDGET_H
#define MAIN_SETTING_WIDGET_H

#include <QWidget>

namespace Ui {
class MainSettingWidget;
}

class MainSettingWidget : public QWidget
{
    Q_OBJECT

public:
    explicit MainSettingWidget(QWidget *parent = nullptr);
    ~MainSettingWidget() override;

private slots:
    // 左侧设置菜单
    void on_aiProviderButton_clicked();
    void on_adminButton_clicked();
    void on_agentSkillsButton_clicked();
    void on_meetingAssistantButton_clicked();
    void on_desktopAssistantButton_clicked();
    void on_communityCenterButton_clicked();
    void on_appearanceButton_clicked();
    void on_channelsButton_clicked();
    void on_toolsButton_clicked();

    // 底部图标按钮
    void on_bottomChatButton_clicked();
    void on_bottomDocsButton_clicked();
    void on_bottomGithubButton_clicked();
    void on_bottomToolsButton_clicked();

    // 右侧下拉框变更
    void on_defaultWindowCombo_currentIndexChanged(int index);
    void on_themeCombo_currentIndexChanged(int index);
    void on_languageCombo_currentIndexChanged(int index);

private:
    void setupLogo();
    void setupStyleSheet();
    void setupMenuGroup();
    void setupConnections();

    Ui::MainSettingWidget *ui;
};

#endif // MAIN_SETTING_WIDGET_H
