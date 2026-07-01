#include "sidebar_footer_widget.h"
#include "icon_button.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QHBoxLayout>
#include <QDesktopServices>
#include <QUrl>

SidebarFooterWidget::SidebarFooterWidget(QWidget *parent)
    : QWidget(parent)
{
    m_githubButton = new IconButton(this);
    m_githubButton->setIconText(QStringLiteral("GH"));
    m_githubButton->setToolTip(tr("View Source Code"));
    connect(m_githubButton, &IconButton::clicked, this, [this]() {
        QDesktopServices::openUrl(QUrl(QStringLiteral("https://github.com/Mintplex-Labs/anything-llm")));
        emit openGitHubRequested();
    });

    m_settingsButton = new IconButton(this);
    m_settingsButton->setIconText(QStringLiteral("⚙"));
    m_settingsButton->setToolTip(tr("Open settings"));
    connect(m_settingsButton, &IconButton::clicked, this, [this]() {
        emit openSettingsRequested();
    });

    auto *layout = new QHBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->addWidget(m_githubButton);
    layout->addStretch();
    layout->addWidget(m_settingsButton);
}

void SidebarFooterWidget::clickGitHubButton()
{
    emit openGitHubRequested();
}

void SidebarFooterWidget::clickSettingsButton()
{
    emit openSettingsRequested();
}
