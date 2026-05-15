#include "thememanager.h"
#include <QApplication>
#include <QFile>

ThemeManager::ThemeManager(QObject *parent)
    : QObject(parent)
    , m_currentTheme(Dark)
{
    loadTheme(Dark);
}

ThemeManager::Theme ThemeManager::currentTheme() const
{
    return m_currentTheme;
}

void ThemeManager::setTheme(Theme theme)
{
    if (m_currentTheme == theme)
        return;
    m_currentTheme = theme;
    loadTheme(theme);
    emit themeChanged(theme);
}

void ThemeManager::loadTheme(Theme theme)
{
    QString resourcePath = (theme == Light) ? QStringLiteral(":/themes/light.qss")
                                            : QStringLiteral(":/themes/dark.qss");
    QFile file(resourcePath);
    if (file.open(QIODevice::ReadOnly)) {
        QByteArray data = file.readAll();
        qApp->setStyleSheet(QString::fromUtf8(data));
    }
}
