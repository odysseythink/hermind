#include "mainwindow.h"
#include "ui_mainwindow.h"

#include "main_chat_widget.h"
#include "main_setting_widget.h"
#include "navigation_manager.h"
#include "auth/auth_manager.h"
#include "workspace_settings_widget.h"
#include "ai_provider_settings_widget.h"
#include "general_appearance_tab.h"
#include "chat_settings_tab.h"
#include "vector_database_tab.h"
#include "members_tab.h"
#include "widgets/agent_config_tab.h"

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

    auto *aiProviderWidget = new AiProviderSettingsWidget(
        AuthManager::instance().apiClient(), this);
    aiProviderWidget->setObjectName(QStringLiteral("aiProviderSettingsWidget"));
    ui->stackedWidget->addWidget(aiProviderWidget);
    connect(aiProviderWidget, &AiProviderSettingsWidget::returnClicked,
            this, []() {
        NavigationManager::instance().goBack();
    });

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

    auto *vectorTab = new VectorDatabaseTab(
        AuthManager::instance().apiClient(), workspaceSettingsWidget);
    workspaceSettingsWidget->setTabWidget(QStringLiteral("vector-database"), vectorTab);

    auto *membersTab = new MembersTab(
        AuthManager::instance().apiClient(), workspaceSettingsWidget);
    membersTab->setObjectName(QStringLiteral("membersTab"));
    workspaceSettingsWidget->setTabWidget(QStringLiteral("members"), membersTab);

    auto *agentConfigTab = new AgentConfigTab(
        AuthManager::instance().apiClient(), workspaceSettingsWidget);
    agentConfigTab->setObjectName(QStringLiteral("agentConfigTab"));
    workspaceSettingsWidget->setTabWidget(QStringLiteral("agent-config"), agentConfigTab);

    // "Configure Agent Skills" leads to the global settings page
    // (web: /settings/agents).
    connect(agentConfigTab, &AgentConfigTab::agentSkillsRequested,
            this, []() {
        NavigationRoute route;
        route.page = NavigationPage::GeneralSettings;
        NavigationManager::instance().navigateTo(route);
    });

    connect(workspaceSettingsWidget, &WorkspaceSettingsWidget::returnClicked,
            this, []() {
        NavigationManager::instance().goBack();
    });

    // After deleting a workspace from GeneralAppearance, leave the dead
    // settings page, refresh the sidebar list, and go back to the default
    // chat (mirrors the web UI redirecting to "/").
    if (chatWidget) {
        connect(generalTab, &GeneralAppearanceTab::workspaceDeleted,
                this, [chatWidget]() {
            chatWidget->refreshWorkspaces();
            NavigationRoute route;
            route.page = NavigationPage::DefaultChat;
            NavigationManager::instance().navigateTo(route);
        });

        // After renaming a workspace, refresh the sidebar list and update
        // the settings header immediately (the web app only refreshes on
        // full reload; the desktop does it live).
        connect(generalTab, &GeneralAppearanceTab::workspaceUpdated,
                this, [chatWidget, workspaceSettingsWidget](const HermindWorkspace &ws) {
            chatWidget->refreshWorkspaces();
            workspaceSettingsWidget->setWorkspaceDisplayName(ws.name());
        });
    }

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

    if (route.page == NavigationPage::GeneralSettings) {
        const QString prefix = QStringLiteral("settings/ai-provider/");
        if (route.settingsPath.startsWith(prefix)) {
            auto *aiWidget = findChild<AiProviderSettingsWidget *>(
                QStringLiteral("aiProviderSettingsWidget"));
            if (aiWidget) {
                ui->stackedWidget->setCurrentWidget(aiWidget);
                aiWidget->setActiveTab(route.settingsPath.mid(prefix.size()));
            }
        }
        return;
    }

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
