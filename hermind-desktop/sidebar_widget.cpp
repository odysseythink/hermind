#include "sidebar_widget.h"
#include "ui_sidebar_widget.h"
#include "sidebar/active_workspaces_widget.h"
#include "sidebar/sidebar_footer_widget.h"
#include "widgets/search_input.h"
#include "widgets/icon_button.h"
#include "widgets/theme_colors.h"
#include "theme_manager.h"
#include "api/hermind_api_client.h"

#include <QPixmap>
#include <QDebug>

SidebarWidget::SidebarWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::SidebarWidget)
{
    ui->setupUi(this);

    ui->searchEdit->setPlaceholderText(tr("搜索"));
    ui->popoutButton->setIconText(QString::fromUtf8("⧉"));
    ui->popoutButton->setToolTip(tr("Pop out sidebar"));
    ui->newWorkspaceButton->setIconText(QStringLiteral("+"));
    ui->newWorkspaceButton->setToolTip(tr("New workspace"));

    connect(ui->newWorkspaceButton, &IconButton::clicked, this, &SidebarWidget::newWorkspaceRequested);
    connect(ui->footer, &SidebarFooterWidget::openSettingsRequested,
            this, &SidebarWidget::openSettingsRequested);

    setupLogo();
    setupStyleSheet();

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &SidebarWidget::setupStyleSheet);
}

SidebarWidget::~SidebarWidget()
{
    delete ui;
}

void SidebarWidget::setApiClient(HermindApiClient *apiClient)
{
    m_apiClient = apiClient;
    ui->activeWorkspaces->setApiClient(apiClient);
}

void SidebarWidget::refreshWorkspaces()
{
    if (ui->activeWorkspaces)
        ui->activeWorkspaces->refresh();
}

void SidebarWidget::setSelectedWorkspace(const QString &slug)
{
    if (ui->activeWorkspaces)
        ui->activeWorkspaces->setSelectedSlug(slug);
}

void SidebarWidget::setSelectedThread(const QString &threadSlug)
{
    if (ui->activeWorkspaces)
        ui->activeWorkspaces->setSelectedThreadSlug(threadSlug);
}

void SidebarWidget::clickSettingsButton()
{
    emit openSettingsRequested();
}

void SidebarWidget::setupLogo()
{
    QPixmap logo(QStringLiteral(":/images/logo.svg"));
    if (!logo.isNull()) {
        ui->logoLabel->setPixmap(logo.scaled(24, 24, Qt::KeepAspectRatio, Qt::SmoothTransformation));
    }
}

void SidebarWidget::setupStyleSheet()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();

    setStyleSheet(QStringLiteral(R"(
        SidebarWidget {
            background-color: %1;
            border: none;
        }
        #brandLabel {
            color: %2;
            font-size: 16px;
            font-weight: 600;
        }
    )").arg(sidebarBg, textPrimary));
}
