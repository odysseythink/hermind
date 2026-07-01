#include "theme_manager.h"
#include "settings_store.h"

#include <QApplication>
#include <QPalette>
#include <QColor>
#include <QSettings>
#include <QStyle>
#include <QStyleFactory>
#include <QProcess>

#ifdef Q_OS_WIN
#include <qt_windows.h>
#endif

// ---------------------------------------------------------------------------
// Singleton
// ---------------------------------------------------------------------------
ThemeManager &ThemeManager::instance()
{
    static ThemeManager mgr;
    return mgr;
}

ThemeManager::ThemeManager()
    : QObject(nullptr)
{
}

// ---------------------------------------------------------------------------
// Initialization
// ---------------------------------------------------------------------------
void ThemeManager::initialize(SettingsStore *settings, QApplication *app)
{
    m_settings = settings;
    m_app = app;

    if (!m_settings)
        return;

    // Read stored values
    m_theme = m_settings->theme();
    m_language = m_settings->language();

    // Apply initial theme
    const QString effective = effectiveTheme();
    if (m_app)
        applyThemeToApplication(effective);
}

// ---------------------------------------------------------------------------
// Theme
// ---------------------------------------------------------------------------
QString ThemeManager::theme() const
{
    return m_theme;
}

void ThemeManager::setTheme(const QString &theme)
{
    if (theme == m_theme)
        return;

    m_theme = theme;

    if (m_settings)
        m_settings->setTheme(theme);

    const QString effective = effectiveTheme();
    if (m_app)
        applyThemeToApplication(effective);

    emit themeChanged(theme);
}

QString ThemeManager::effectiveTheme() const
{
    if (m_theme == QLatin1String("dark"))
        return QStringLiteral("dark");
    if (m_theme == QLatin1String("light"))
        return QStringLiteral("light");
    // "system" or any unknown value → detect
    return detectSystemTheme();
}

bool ThemeManager::isDarkMode() const
{
    return effectiveTheme() == QLatin1String("dark");
}

// ---------------------------------------------------------------------------
// Language
// ---------------------------------------------------------------------------
QString ThemeManager::language() const
{
    return m_language;
}

void ThemeManager::setLanguage(const QString &language)
{
    if (language == m_language)
        return;

    m_language = language;

    if (m_settings)
        m_settings->setLanguage(language);

    emit languageChanged(language);
}

// ---------------------------------------------------------------------------
// System theme detection
// ---------------------------------------------------------------------------
QString ThemeManager::detectSystemTheme()
{
#ifdef Q_OS_WIN
    // Windows: read HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize\AppsUseLightTheme
    // 0 = dark, 1 = light, missing = light
    QSettings reg(QStringLiteral("HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize"),
                  QSettings::NativeFormat);
    const int appsUseLight = reg.value(QStringLiteral("AppsUseLightTheme"), 1).toInt();
    return appsUseLight == 0 ? QStringLiteral("dark") : QStringLiteral("light");
#elif defined(Q_OS_MAC)
    // macOS: check AppleInterfaceStyle via defaults
    QProcess proc;
    proc.start(QStringLiteral("defaults"), {QStringLiteral("read"), QStringLiteral("-g"), QStringLiteral("AppleInterfaceStyle")});
    proc.waitForFinished(500);
    const QString output = QString::fromUtf8(proc.readAllStandardOutput()).trimmed();
    return output == QLatin1String("Dark") ? QStringLiteral("dark") : QStringLiteral("light");
#else
    // Linux: check gsettings for GNOME, KDE, etc.
    // Try gsettings first (GNOME / GTK)
    QProcess proc;
    proc.start(QStringLiteral("gsettings"), {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"), QStringLiteral("gtk-theme")});
    proc.waitForFinished(500);
    const QString output = QString::fromUtf8(proc.readAllStandardOutput()).toLower();
    if (output.contains(QLatin1String("dark")))
        return QStringLiteral("dark");

    // Try KDE
    proc.start(QStringLiteral("kreadconfig5"), {QStringLiteral("--file"), QStringLiteral("kdeglobals"),
                 QStringLiteral("--group"), QStringLiteral("General"), QStringLiteral("--key"), QStringLiteral("ColorScheme")});
    proc.waitForFinished(500);
    const QString kdeOutput = QString::fromUtf8(proc.readAllStandardOutput()).toLower();
    if (kdeOutput.contains(QLatin1String("dark")))
        return QStringLiteral("dark");

    return QStringLiteral("light");
#endif
}

// ---------------------------------------------------------------------------
// Palette application
// ---------------------------------------------------------------------------
void ThemeManager::applyThemeToApplication(const QString &effectiveTheme)
{
    if (!m_app)
        return;

    // Use Fusion style for consistent cross-platform look
    m_app->setStyle(QStyleFactory::create(QStringLiteral("Fusion")));

    if (effectiveTheme == QLatin1String("dark")) {
        // --- Dark palette ---
        QPalette darkPalette;

        const QColor darkBg(30, 30, 30);
        const QColor darkAlt(38, 38, 38);
        const QColor darkBase(25, 25, 25);
        const QColor darkText(220, 220, 220);
        const QColor darkDisabled(128, 128, 128);
        const QColor darkHighlight(42, 130, 218);
        const QColor darkHighlightedText(255, 255, 255);
        const QColor darkButton(53, 53, 53);
        const QColor darkButtonText(220, 220, 220);
        const QColor darkToolTipBase(60, 60, 60);
        const QColor darkToolTipText(220, 220, 220);
        const QColor darkLink(42, 130, 218);
        const QColor darkLinkVisited(128, 80, 180);

        darkPalette.setColor(QPalette::Window, darkBg);
        darkPalette.setColor(QPalette::WindowText, darkText);
        darkPalette.setColor(QPalette::Base, darkBase);
        darkPalette.setColor(QPalette::AlternateBase, darkAlt);
        darkPalette.setColor(QPalette::ToolTipBase, darkToolTipBase);
        darkPalette.setColor(QPalette::ToolTipText, darkToolTipText);
        darkPalette.setColor(QPalette::Text, darkText);
        darkPalette.setColor(QPalette::Disabled, QPalette::Text, darkDisabled);
        darkPalette.setColor(QPalette::Button, darkButton);
        darkPalette.setColor(QPalette::ButtonText, darkButtonText);
        darkPalette.setColor(QPalette::Disabled, QPalette::ButtonText, darkDisabled);
        darkPalette.setColor(QPalette::BrightText, Qt::red);
        darkPalette.setColor(QPalette::Link, darkLink);
        darkPalette.setColor(QPalette::LinkVisited, darkLinkVisited);
        darkPalette.setColor(QPalette::Highlight, darkHighlight);
        darkPalette.setColor(QPalette::HighlightedText, darkHighlightedText);
        darkPalette.setColor(QPalette::Disabled, QPalette::HighlightedText, darkDisabled);

        // ComboBox / LineEdit
        darkPalette.setColor(QPalette::Active, QPalette::Button, darkButton);
        darkPalette.setColor(QPalette::Disabled, QPalette::ButtonText, darkDisabled);
        darkPalette.setColor(QPalette::Disabled, QPalette::WindowText, darkDisabled);
        darkPalette.setColor(QPalette::Disabled, QPalette::Text, darkDisabled);
        darkPalette.setColor(QPalette::Disabled, QPalette::Light, darkButton);

        m_app->setPalette(darkPalette);

        // Force dark style sheet additions for native dialogs
        m_app->setStyleSheet(QStringLiteral(
            "QToolTip { color: #dcdcdc; background-color: #3c3c3c; border: 1px solid #5a5a5a; }"
            "QMenu { background-color: #2d2d2d; color: #dcdcdc; border: 1px solid #5a5a5a; }"
            "QMenu::item:selected { background-color: #2a82da; }"
        ));
    } else {
        // --- Light palette (Fusion default-like) ---
        m_app->setPalette(QApplication::style()->standardPalette());
        m_app->setStyleSheet(QString());
    }
}
