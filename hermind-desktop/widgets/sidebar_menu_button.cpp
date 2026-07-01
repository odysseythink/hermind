#include "sidebar_menu_button.h"
#include "theme_colors.h"
#include "theme_style_helper.h"

SidebarMenuButton::SidebarMenuButton(const QString &text, QWidget *parent)
    : QPushButton(text, parent)
{
    setCheckable(true);
    setFlat(true);
    setCursor(Qt::PointingHandCursor);
    setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Fixed);
    setMinimumHeight(36);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *btn = qobject_cast<SidebarMenuButton *>(w);
        if (btn)
            btn->applyStyle(dark);
    }, this);
}

void SidebarMenuButton::applyStyle(bool dark)
{
    const QString hover = ThemeColors::hoverBackground(dark).name();
    const QString selected = ThemeColors::selectedBackground(dark).name();
    const QString text = ThemeColors::textPrimary(dark).name();

    setStyleSheet(QStringLiteral(
        "QPushButton {"
        "  text-align: left;"
        "  border: none;"
        "  border-radius: 8px;"
        "  background-color: transparent;"
        "  color: %1;"
        "  font-size: 14px;"
        "  padding: 8px 12px;"
        "}"
        "QPushButton:hover {"
        "  background-color: %2;"
        "}"
        "QPushButton:checked {"
        "  background-color: %3;"
        "  color: %1;"
        "}"
    ).arg(text, hover, selected));
}
