#include "main_chat_widget.h"
#include "ui_main_chat_widget.h"
#include "widgets/icon_button.h"
#include "widgets/theme_colors.h"
#include "theme_manager.h"
#include "sidebar_widget.h"
#include "new_workspace_dialog.h"
#include "auth/auth_manager.h"
#include "api/hermind_api_client.h"
#include "widgets/chat_container_widget.h"
#include "navigation/navigation_manager.h"
#include "navigation/navigation_route.h"

#include <QDebug>
#include <QBoxLayout>

namespace {

QBoxLayout *findLayoutContaining(QWidget *widget)
{
    QWidget *top = widget->window();
    if (!top)
        top = widget;

    const QList<QLayout *> layouts = top->findChildren<QLayout *>();
    for (QLayout *layout : layouts) {
        for (int i = 0; i < layout->count(); ++i) {
            if (layout->itemAt(i)->widget() == widget)
                return qobject_cast<QBoxLayout *>(layout);
        }
    }
    return nullptr;
}

} // namespace

MainChatWidget::MainChatWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainChatWidget)
{
    ui->setupUi(this);

    replaceSidebar();
    setupChatContainer();
    replaceToolButtons();
    setupStyleSheet();

    // Header settings button → GeneralSettings (same entry as the sidebar's).
    connect(ui->headerSettingsButton, &QToolButton::clicked,
            this, &MainChatWidget::bottomSettingClicked);

    connect(&NavigationManager::instance(), &NavigationManager::currentRouteChanged,
            this, &MainChatWidget::onRouteChanged);
    updateSidebarSelection(NavigationManager::instance().currentRoute());
}

MainChatWidget::~MainChatWidget()
{
    delete ui;
}

void MainChatWidget::refreshWorkspaces()
{
    if (m_sidebar)
        m_sidebar->refreshWorkspaces();
}

void MainChatWidget::replaceSidebar()
{
    m_sidebar = new SidebarWidget(this);
    m_sidebar->setObjectName(QStringLiteral("sidebarWidget"));
    m_sidebar->setMinimumWidth(260);
    m_sidebar->setMaximumWidth(260);

    if (HermindApiClient *client = AuthManager::instance().apiClient())
        m_sidebar->setApiClient(client);

    connect(m_sidebar, &SidebarWidget::openSettingsRequested,
            this, &MainChatWidget::bottomSettingClicked);
    connect(m_sidebar, &SidebarWidget::newWorkspaceRequested,
            this, &MainChatWidget::onNewWorkspaceRequested);

    QBoxLayout *mainLayout = qobject_cast<QBoxLayout *>(ui->horizontalLayout_2);
    if (mainLayout && ui->sidebarFrame) {
        int idx = -1;
        for (int i = 0; i < mainLayout->count(); ++i) {
            if (mainLayout->itemAt(i)->widget() == ui->sidebarFrame) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            mainLayout->removeWidget(ui->sidebarFrame);
            mainLayout->insertWidget(idx, m_sidebar);
        }
    }

    // The old sidebar UI is fully replaced by SidebarWidget.
    delete ui->sidebarFrame;
    ui->sidebarFrame = nullptr;
}

void MainChatWidget::setupChatContainer()
{
    if (!ui->chatContainerPlaceholder)
        return;

    m_chatContainer = new ChatContainerWidget(AuthManager::instance().apiClient(), this);
    m_chatContainer->setObjectName(QStringLiteral("chatContainerWidget"));

    QVBoxLayout *layout = qobject_cast<QVBoxLayout *>(ui->chatFrame->layout());
    if (layout) {
        int idx = -1;
        for (int i = 0; i < layout->count(); ++i) {
            if (layout->itemAt(i)->widget() == ui->chatContainerPlaceholder) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            layout->removeWidget(ui->chatContainerPlaceholder);
            layout->insertWidget(idx, m_chatContainer);
        }
    }

    ui->chatContainerPlaceholder->deleteLater();
    ui->chatContainerPlaceholder = nullptr;
}

void MainChatWidget::replaceToolButtons()
{
    auto replaceOne = [this](const QString &name, const QString &iconText) {
        QToolButton *oldBtn = findChild<QToolButton *>(name);
        if (!oldBtn)
            return;

        IconButton *newBtn = new IconButton(this);
        newBtn->setObjectName(name);
        newBtn->setIconText(iconText);

        QBoxLayout *layout = findLayoutContaining(oldBtn);
        if (layout) {
            int idx = -1;
            for (int i = 0; i < layout->count(); ++i) {
                if (layout->itemAt(i)->widget() == oldBtn) {
                    idx = i;
                    break;
                }
            }
            if (idx >= 0) {
                layout->removeWidget(oldBtn);
                layout->insertWidget(idx, newBtn);
            }
        }

        oldBtn->deleteLater();

        if (name == QLatin1String("headerSettingsButton")) ui->headerSettingsButton = newBtn;
    };

    replaceOne(QStringLiteral("headerSettingsButton"), QString::fromUtf8("⚙"));
}

void MainChatWidget::setupStyleSheet()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString textSecondary = ThemeColors::textSecondary(dark).name();
    const QString hoverBg = ThemeColors::hoverBackground(dark).name();
    const QString selectedBg = ThemeColors::selectedBackground(dark).name();

    setStyleSheet(QStringLiteral(R"(
        MainChatWidget {
            background-color: %1;
        }

        #sidebarWidget {
            background-color: %2;
            border: none;
        }

        #chatFrame {
            background-color: %1;
            border: none;
        }

        #brandLabel {
            color: %3;
            font-size: 16px;
            font-weight: 600;
        }

        #workspaceLabel {
            color: %3;
            font-size: 13px;
            font-weight: 500;
        }

        #workspaceIcon {
            color: %4;
            font-size: 12px;
        }

        #threadList {
            background-color: transparent;
            border: none;
            outline: none;
        }

        #threadList::item {
            color: %3;
            font-size: 13px;
            padding: 8px 12px;
            border-radius: 6px;
        }

        #threadList::item:selected {
            background-color: %6;
            color: %3;
        }

        #threadList::item:hover {
            background-color: %5;
        }

        #newThreadButton, #assistantChatsButton {
            background-color: %1;
            border: 1px solid %5;
            border-radius: 8px;
            padding: 8px 12px;
            color: %3;
            font-size: 13px;
            text-align: left;
        }

        #newThreadButton:hover, #assistantChatsButton:hover {
            background-color: %5;
        }

        #workspaceNameLabel {
            color: %4;
            font-size: 14px;
            font-weight: 500;
        }

        #versionLabel {
            color: #D4A017;
            font-size: 12px;
            font-weight: 500;
        }

        #welcomeLabel {
            color: %3;
            font-size: 28px;
            font-weight: 500;
        }

        #inputFrame {
            background-color: %1;
            border: 1px solid %5;
            border-radius: 16px;
        }

        #messageEdit {
            background-color: transparent;
            border: none;
            color: %3;
            font-size: 14px;
        }

        #messageEdit::placeholder {
            color: %4;
        }

        #toolsButton {
            background-color: transparent;
            border: none;
            color: %4;
            font-size: 13px;
            padding: 4px 8px;
        }

        #toolsButton:hover {
            background-color: %5;
            border-radius: 6px;
        }

        #createAgentButton, #editWorkspaceButton, #uploadFileButton {
            background-color: %5;
            border: none;
            border-radius: 16px;
            padding: 8px 16px;
            color: %3;
            font-size: 13px;
        }

        #createAgentButton:hover, #editWorkspaceButton:hover, #uploadFileButton:hover {
            background-color: %5;
        }
    )").arg(windowBg, sidebarBg, textPrimary, textSecondary, hoverBg, selectedBg));
}

void MainChatWidget::onNewWorkspaceRequested()
{
    HermindApiClient *client = AuthManager::instance().apiClient();
    if (!client)
        return;

    NewWorkspaceDialog dialog(this);
    dialog.setApiClient(client);
    connect(&dialog, &NewWorkspaceDialog::workspaceCreated,
            this, [](const HermindWorkspace &workspace) {
        NavigationRoute route;
        route.page = NavigationPage::WorkspaceChat;
        route.workspaceSlug = workspace.slug();
        NavigationManager::instance().navigateTo(route);
    });
    dialog.exec();
}

void MainChatWidget::onRouteChanged(const NavigationRoute &route)
{
    updateSidebarSelection(route);

    if (!m_chatContainer)
        return;

    switch (route.page) {
    case NavigationPage::WorkspaceChat:
        m_chatContainer->setWorkspace(route.workspaceSlug, route.workspaceSlug);
        m_chatContainer->setThreadSlug(route.threadSlug);
        break;
    default:
        m_chatContainer->setWorkspace(QString(), QString());
        m_chatContainer->setThreadSlug(QString());
        break;
    }
}

void MainChatWidget::updateSidebarSelection(const NavigationRoute &route)
{
    if (!m_sidebar)
        return;

    switch (route.page) {
    case NavigationPage::WorkspaceChat:
    case NavigationPage::WorkspaceSettings:
        m_sidebar->setSelectedWorkspace(route.workspaceSlug);
        m_sidebar->setSelectedThread(route.threadSlug);
        break;
    default:
        m_sidebar->setSelectedWorkspace(QString());
        m_sidebar->setSelectedThread(QString());
        break;
    }

    m_sidebar->refreshWorkspaces();
}
