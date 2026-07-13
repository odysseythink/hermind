#include "mainwindow.h"
#include "ui_mainwindow.h"

#include "main_chat_widget.h"
#include "main_setting_widget.h"
#include "navigation_manager.h"
#include "auth/auth_manager.h"
#include "workspace_settings_widget.h"
#include "general_appearance_tab.h"
#include "chat_settings_tab.h"

#include <QDebug>

MainWindow::MainWindow(QWidget *parent)
    : QMainWindow(parent)
    , ui(new Ui::MainWindow)
{
    ui->setupUi(this);

    setWindowTitle(tr("Hermind Desktop"));

    auto *chatWidget = qobject_cast<MainChatWidget*>(ui->stackedWidget->widget(0));
    auto *settingWidget = qobject_cast<MainSettingWidget*>(ui->stackedWidget->widget(1));

    if (chatWidget) {
        // MainChatWidget handles both the default chat and workspace chat
        // routes internally (see MainChatWidget::onRouteChanged).
        registerPage(NavigationPage::DefaultChat, chatWidget);
        registerPage(NavigationPage::WorkspaceChat, chatWidget);
        connect(chatWidget, &MainChatWidget::bottomSettingClicked,
                this, []() {
            NavigationRoute route;
            route.page = NavigationPage::GeneralSettings;
            NavigationManager::instance().navigateTo(route);
        });
    }

    if (settingWidget) {
        registerPage(NavigationPage::GeneralSettings, settingWidget);
        connect(settingWidget, &MainSettingWidget::bottomReturnClicked,
                this, []() {
            NavigationManager::instance().goBack();
        });
    }

    auto *workspaceSettingsWidget = new WorkspaceSettingsWidget(
        AuthManager::instance().apiClient(), this);
    workspaceSettingsWidget->setObjectName(QStringLiteral("workspaceSettingsWidget"));
    ui->stackedWidget->addWidget(workspaceSettingsWidget);
    registerPage(NavigationPage::WorkspaceSettings, workspaceSettingsWidget);

    auto *generalTab = new GeneralAppearanceTab(
        AuthManager::instance().apiClient(), workspaceSettingsWidget);
    workspaceSettingsWidget->setTabWidget(QStringLiteral("general-appearance"), generalTab);

    auto *chatTab = new ChatSettingsTab(
        AuthManager::instance().apiClient(), workspaceSettingsWidget);
    workspaceSettingsWidget->setTabWidget(QStringLiteral("chat"), chatTab);

    connect(workspaceSettingsWidget, &WorkspaceSettingsWidget::returnClicked,
            this, []() {
        NavigationManager::instance().goBack();
    });

    connect(&NavigationManager::instance(), &NavigationManager::currentRouteChanged,
            this, &MainWindow::onCurrentRouteChanged);

    onCurrentRouteChanged(NavigationManager::instance().currentRoute());
}

void MainWindow::registerPage(NavigationPage page, QWidget *widget)
{
    m_pageRegistry.insert(page, widget);
}

int MainWindow::pageIndex(NavigationPage page) const
{
    QWidget *widget = m_pageRegistry.value(page, nullptr);
    if (!widget)
        return -1;
    return ui->stackedWidget->indexOf(widget);
}

void MainWindow::onCurrentRouteChanged(const NavigationRoute &route)
{
    const int index = pageIndex(route.page);
    if (index < 0) {
        qWarning() << "No widget registered for page" << static_cast<int>(route.page);
        return;
    }

    ui->stackedWidget->setCurrentIndex(index);

    if (route.page == NavigationPage::WorkspaceSettings) {
        auto *w = qobject_cast<WorkspaceSettingsWidget *>(
            m_pageRegistry.value(NavigationPage::WorkspaceSettings));
        if (w) {
            w->setWorkspaceSlug(route.workspaceSlug);
            w->setActiveTab(route.settingsPath.isEmpty()
                                ? QStringLiteral("general-appearance")
                                : route.settingsPath);
        }
    }
}

MainWindow::~MainWindow()
{
    delete ui;
}
