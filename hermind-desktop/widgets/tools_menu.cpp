#include "tools_menu.h"
#include "slash_commands_tab.h"
#include "theme_manager.h"
#include "theme_colors.h"

#include <QTabBar>
#include <QStackedWidget>
#include <QVBoxLayout>
#include <QKeyEvent>
#include <QFocusEvent>

ToolsMenu::ToolsMenu(QWidget *parent)
    : QWidget(parent, Qt::Popup | Qt::FramelessWindowHint)
{
    setFixedWidth(400);
    setMaximumHeight(360);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(8);

    m_tabBar = new QTabBar(this);
    m_tabBar->setExpanding(false);
    m_tabBar->addTab(QStringLiteral("Slash 命令"));
    m_tabBar->addTab(QStringLiteral("Agent 技能")); // placeholder
    m_tabBar->setTabEnabled(1, false); // disable Agent Skills for now

    m_tabs = new QStackedWidget(this);

    m_slashTab = new SlashCommandsTab(this);
    m_tabs->addWidget(m_slashTab);
    m_tabs->addWidget(new QWidget(this)); // placeholder for Agent Skills

    layout->addWidget(m_tabBar);
    layout->addWidget(m_tabs, 1);

    connect(m_tabBar, &QTabBar::currentChanged,
            m_tabs, &QStackedWidget::setCurrentIndex);

    connect(m_slashTab, &SlashCommandsTab::commandSelected,
            this, [this](const QString &text, const QString &mode) {
        if (m_callback)
            m_callback(text, mode);
        hide();
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void ToolsMenu::setSendCommandCallback(std::function<void(const QString &, const QString &)> cb)
{
    m_callback = std::move(cb);
    m_slashTab->setSendCommandCallback(m_callback);
}

void ToolsMenu::showAbove(QWidget *anchor)
{
    adjustSize();
    QPoint pos = anchor->mapToGlobal(QPoint(0, -height() - 4));
    move(pos);
    show();
    setFocus();
}

void ToolsMenu::showBelow(QWidget *anchor)
{
    adjustSize();
    QPoint pos = anchor->mapToGlobal(QPoint(0, anchor->height() + 4));
    move(pos);
    show();
    setFocus();
}

void ToolsMenu::focusOutEvent(QFocusEvent *event)
{
    QWidget::focusOutEvent(event);
    if (event->reason() != Qt::ActiveWindowFocusReason) {
        hide();
        emit closed();
    }
}

void ToolsMenu::keyPressEvent(QKeyEvent *event)
{
    if (event->key() == Qt::Key_Escape) {
        hide();
        emit closed();
        return;
    }
    if (event->key() == Qt::Key_Up || event->key() == Qt::Key_Down) {
        m_slashTab->handleArrowKey(event->key());
        return;
    }
    if (event->key() == Qt::Key_Return || event->key() == Qt::Key_Enter) {
        m_slashTab->activateHighlighted();
        return;
    }
    QWidget::keyPressEvent(event);
}

void ToolsMenu::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "ToolsMenu { background-color: %1; border: 1px solid %2; border-radius: 12px; }"
    ).arg(ThemeColors::cardBackground(dark).name(),
          ThemeColors::border(dark).name()));
}
