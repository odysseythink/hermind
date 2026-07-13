#include "agent_menu.h"
#include "prompt_input.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QPushButton>
#include <QVBoxLayout>
#include <QFocusEvent>

AgentMenu::AgentMenu(QWidget *parent)
    : QWidget(parent, Qt::Popup | Qt::FramelessWindowHint)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(4, 4, 4, 4);
    layout->setSpacing(2);

    m_agentItem = new QPushButton(this);
    m_agentItem->setText(QStringLiteral("@agent — 启动 Agent 会话"));
    m_agentItem->setCursor(Qt::PointingHandCursor);
    m_agentItem->setFlat(true);
    m_agentItem->setStyleSheet(QStringLiteral(
        "QPushButton { text-align: left; padding: 8px 12px; border-radius: 8px; }"
        "QPushButton:hover { background-color: palette(highlight); }"
    ));
    layout->addWidget(m_agentItem);

    connect(m_agentItem, &QPushButton::clicked, this, [this]() {
        if (m_callback)
            m_callback(QStringLiteral("@agent "), WriteMode::Prepend);
        hide();
        emit agentSelected();
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void AgentMenu::showAt(const QPoint &globalPos)
{
    adjustSize();
    move(globalPos);
    show();
    setFocus();
}

void AgentMenu::setSendCommandCallback(std::function<void(const QString &, const QString &)> callback)
{
    m_callback = callback;
}

void AgentMenu::focusOutEvent(QFocusEvent *event)
{
    QWidget::focusOutEvent(event);
    if (event->reason() != Qt::ActiveWindowFocusReason)
        hide();
}

void AgentMenu::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "AgentMenu { background-color: %1; border: 1px solid %2; border-radius: 10px; }"
    ).arg(ThemeColors::cardBackground(dark).name(),
          ThemeColors::border(dark).name()));
}
