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

signals:
    void bottomReturnClicked();

private slots:
    void on_aiProviderButton_clicked();
    void on_adminButton_clicked();
    void on_agentSkillsButton_clicked();
    void on_meetingAssistantButton_clicked();
    void on_desktopAssistantButton_clicked();
    void on_communityCenterButton_clicked();
    void on_appearanceButton_clicked();
    void on_channelsButton_clicked();
    void on_toolsButton_clicked();

    void on_bottomReturnButton_clicked();

    void on_defaultWindowCombo_currentIndexChanged(int index);
    void on_themeCombo_currentIndexChanged(int index);
    void on_languageCombo_currentIndexChanged(int index);

private:
    void setupLogo();
    void setupStyleSheet();
    void setupMenuGroup();
    void setupConnections();
    void replaceMenuButtons();
    void replaceSeparator();
    void replaceContentFrame();
    void rebuildSettingRows();

    Ui::MainSettingWidget *ui;
};

#endif // MAIN_SETTING_WIDGET_H
