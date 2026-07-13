#include "quick_actions.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QHBoxLayout>
#include <QPushButton>

QuickActions::QuickActions(QWidget *parent)
    : QWidget(parent)
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setSpacing(8);

    m_createAgentBtn = new QPushButton(QStringLiteral("🤖 创建代理"), this);
    m_editWorkspaceBtn = new QPushButton(QStringLiteral("⚙ 编辑工作区"), this);
    m_uploadBtn = new QPushButton(QStringLiteral("📁 上传文件"), this);

    for (auto *btn : { m_createAgentBtn, m_editWorkspaceBtn, m_uploadBtn }) {
        btn->setFlat(true);
        btn->setCursor(Qt::PointingHandCursor);
        layout->addWidget(btn);
    }

    connect(m_createAgentBtn, &QPushButton::clicked, this, &QuickActions::createAgentClicked);
    connect(m_editWorkspaceBtn, &QPushButton::clicked, this, &QuickActions::editWorkspaceClicked);
    connect(m_uploadBtn, &QPushButton::clicked, this, &QuickActions::uploadDocumentClicked);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void QuickActions::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString bg = dark ? QStringLiteral("#3f3f46") : QStringLiteral("#e2e8f0");
    const QString fg = dark ? QStringLiteral("#e4e4e7") : QStringLiteral("#334155");
    setStyleSheet(QStringLiteral(
        "QPushButton { background-color: %1; color: %2; border-radius: 16px; padding: 8px 16px; font-size: 13px; }"
        "QPushButton:hover { opacity: 0.8; }"
    ).arg(bg, fg));
}
