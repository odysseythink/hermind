#include "tools_menu.h"
#include "slash_commands_tab.h"
#include "theme_manager.h"
#include "theme_colors.h"

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

    m_slashTab = new SlashCommandsTab(this);
    layout->addWidget(m_slashTab, 1);

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
