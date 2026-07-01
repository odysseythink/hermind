#include "theme_style_helper.h"
#include "theme_manager.h"

ThemeStyleHelper::ThemeStyleHelper(QWidget *owner, StyleUpdater updater, QObject *parent)
    : QObject(parent)
    , m_owner(owner)
    , m_updater(std::move(updater))
{
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &ThemeStyleHelper::onThemeChanged);

    if (m_owner && m_updater)
        m_updater(m_owner, ThemeManager::instance().isDarkMode());
}

void ThemeStyleHelper::onThemeChanged(const QString &theme)
{
    Q_UNUSED(theme)
    if (m_owner && m_updater)
        m_updater(m_owner, ThemeManager::instance().isDarkMode());
}
