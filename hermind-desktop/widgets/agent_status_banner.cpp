#include "agent_status_banner.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include <QLabel>
#include <QVBoxLayout>

AgentStatusBanner::AgentStatusBanner(QWidget *parent)
    : QWidget(parent)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(12, 8, 12, 8);
    m_label = new QLabel(this);
    m_label->setWordWrap(true);
    m_label->setStyleSheet(QStringLiteral("font-size: 13px;"));
    layout->addWidget(m_label);
    setVisible(false);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void AgentStatusBanner::showStatus(const QString &status)
{
    m_label->setText(QStringLiteral("🤖 %1").arg(status));
    show();
}

void AgentStatusBanner::showClarification(const QString &question)
{
    m_label->setText(QStringLiteral("❓ %1").arg(question));
    show();
}

void AgentStatusBanner::hideBanner()
{
    hide();
}

void AgentStatusBanner::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "AgentStatusBanner { background-color: %1; border-radius: 8px; }"
    ).arg(ThemeColors::cardBackground(dark).name()));
}
