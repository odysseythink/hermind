#include "Theme.h"

Theme::Theme(QObject *parent)
    : QObject(parent)
{
}

void Theme::setIsDark(bool dark)
{
    if (m_isDark != dark) {
        m_isDark = dark;
        emit isDarkChanged();
    }
}
