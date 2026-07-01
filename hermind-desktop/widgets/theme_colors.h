#ifndef THEME_COLORS_H
#define THEME_COLORS_H

#include <QColor>

class ThemeColors
{
public:
    // Backgrounds
    static QColor windowBackground(bool dark);
    static QColor sidebarBackground(bool dark);
    static QColor cardBackground(bool dark);
    static QColor inputBackground(bool dark);
    static QColor hoverBackground(bool dark);
    static QColor selectedBackground(bool dark);

    // Text
    static QColor textPrimary(bool dark);
    static QColor textSecondary(bool dark);
    static QColor textDisabled(bool dark);

    // Accents
    static QColor primary(bool dark);
    static QColor primaryHover(bool dark);
    static QColor border(bool dark);
    static QColor separator(bool dark);
};

#endif // THEME_COLORS_H
