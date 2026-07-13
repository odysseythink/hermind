#include "slash_commands_tab.h"
#include "prompt_input.h"
#include "theme_manager.h"
#include "theme_colors.h"

#include <QListWidget>
#include <QVBoxLayout>

SlashCommandsTab::SlashCommandsTab(QWidget *parent)
    : QWidget(parent)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(this);
    m_list->setFrameShape(QFrame::NoFrame);
    m_list->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    layout->addWidget(m_list);

    // Predefined slash commands (matches frontend SlashPresets/constants.js)
    m_commands = {
        { QStringLiteral("/reset — 重置聊天"),             QStringLiteral("/reset"), WriteMode::Replace },
        { QStringLiteral("/new — 清除上下文并开始新话题"), QStringLiteral("/new"),   WriteMode::Replace },
        { QStringLiteral("/help — 显示帮助"),              QStringLiteral("/help"),  WriteMode::Replace },
    };

    for (const SlashCommand &cmd : m_commands) {
        QListWidgetItem *item = new QListWidgetItem(cmd.label);
        item->setData(Qt::UserRole, cmd.text);
        item->setData(Qt::UserRole + 1, cmd.writeMode);
        m_list->addItem(item);
    }

    connect(m_list, &QListWidget::itemClicked, this, [this](QListWidgetItem *item) {
        QString text = item->data(Qt::UserRole).toString();
        QString mode = item->data(Qt::UserRole + 1).toString();
        // The parent (ToolsMenu) owns the send-command callback and listens
        // to this signal; invoking a callback here too would double-send.
        emit commandSelected(text, mode);
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void SlashCommandsTab::handleArrowKey(int key)
{
    int current = m_list->currentRow();
    int count = m_list->count();
    if (count == 0) return;

    int next;
    if (key == Qt::Key_Down) {
        next = (current < count - 1) ? current + 1 : 0;
    } else { // Key_Up
        next = (current > 0) ? current - 1 : count - 1;
    }
    m_list->setCurrentRow(next);
}

void SlashCommandsTab::activateHighlighted()
{
    QListWidgetItem *item = m_list->currentItem();
    if (item)
        emit m_list->itemClicked(item);
}

void SlashCommandsTab::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    m_list->setStyleSheet(QStringLiteral(
        "QListWidget { background: transparent; }"
        "QListWidget::item { padding: 8px 12px; border-radius: 6px; color: %1; }"
        "QListWidget::item:selected { background-color: %2; }"
        "QListWidget::item:hover { background-color: %2; }"
    ).arg(ThemeColors::textPrimary(dark).name(),
          ThemeColors::hoverBackground(dark).name()));
}
