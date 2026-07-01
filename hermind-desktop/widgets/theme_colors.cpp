#include "theme_colors.h"

QColor ThemeColors::windowBackground(bool dark)
{
    return dark ? QColor(30, 30, 30) : QColor(255, 255, 255);
}

QColor ThemeColors::sidebarBackground(bool dark)
{
    return dark ? QColor(38, 38, 38) : QColor(232, 237, 242);
}

QColor ThemeColors::cardBackground(bool dark)
{
    return dark ? QColor(45, 45, 45) : QColor(255, 255, 255);
}

QColor ThemeColors::inputBackground(bool dark)
{
    return dark ? QColor(53, 53, 53) : QColor(255, 255, 255);
}

QColor ThemeColors::hoverBackground(bool dark)
{
    return dark ? QColor(60, 60, 60) : QColor(221, 227, 233);
}

QColor ThemeColors::selectedBackground(bool dark)
{
    return dark ? QColor(42, 80, 120) : QColor(214, 232, 247);
}

QColor ThemeColors::textPrimary(bool dark)
{
    return dark ? QColor(220, 220, 220) : QColor(31, 41, 55);
}

QColor ThemeColors::textSecondary(bool dark)
{
    return dark ? QColor(160, 160, 160) : QColor(107, 114, 128);
}

QColor ThemeColors::textDisabled(bool dark)
{
    return dark ? QColor(128, 128, 128) : QColor(156, 163, 175);
}

QColor ThemeColors::primary(bool dark)
{
    Q_UNUSED(dark)
    return QColor(91, 141, 239);
}

QColor ThemeColors::primaryHover(bool dark)
{
    Q_UNUSED(dark)
    return QColor(74, 125, 224);
}

QColor ThemeColors::border(bool dark)
{
    return dark ? QColor(80, 80, 80) : QColor(229, 231, 235);
}

QColor ThemeColors::separator(bool dark)
{
    return dark ? QColor(80, 80, 80) : QColor(229, 231, 235);
}
